// Package secretui runs a one-shot localhost HTTP server that lets a human
// enter secret values in a browser without the values ever passing through
// the calling agent's stdout/stderr.
package secretui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed assets/index.html assets/app.js assets/app.css
var assets embed.FS

const (
	maxBodyBytes = 64 * 1024
	csp          = "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

type RequiredSecret struct{ Name, Description string }

type EntryStatus string

const (
	EntrySet     EntryStatus = "set"
	EntryMissing EntryStatus = "missing" // in config but absent from the store
)

type Entry struct {
	Name      string      `json:"name"`
	Status    EntryStatus `json:"status"`
	UpdatedAt *time.Time  `json:"updated_at,omitempty"`
}

type Request struct {
	Namespace   string
	Scope       string // "global" | "local"
	StorageNote string // shown verbatim on the page
	Required    []RequiredSecret
	Existing    []Entry // all known names in the namespace, sorted by name
	Save        func(name string, value []byte) (warning string, err error)
	Timeout     time.Duration
	OpenBrowser func(url string) error // nil = do not open
	OnReady     func(url string)       // called once listening, before opening the browser
	Log         io.Writer              // hints such as browser-open failure; nil = discard
}

type Result struct {
	Name    string `json:"name"`
	Saved   bool   `json:"saved"`
	Warning string `json:"warning,omitempty"`
	Error   string `json:"error,omitempty"`
}

type Status string

const (
	StatusSaved     Status = "saved"
	StatusCancelled Status = "cancelled"
	StatusTimeout   Status = "timeout"
)

type Outcome struct {
	Status  Status
	Results []Result
}

type server struct {
	req    Request
	token  string
	host   string
	origin string
	tmpl   *template.Template

	mu      sync.Mutex
	results map[string]Result
	allowed map[string]bool
	finish  chan Status
	saved   bool // set under mu once a save completed everything
}

// Run starts the server and blocks until all required secrets are saved, the
// user cancels, ctx is cancelled, or the timeout elapses. Only startup
// failures are returned as errors.
func Run(ctx context.Context, req Request) (Outcome, error) {
	if req.Log == nil {
		req.Log = io.Discard
	}
	if req.Timeout <= 0 {
		req.Timeout = 5 * time.Minute
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return Outcome{}, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(buf)

	tmpl, err := template.ParseFS(assets, "assets/index.html")
	if err != nil {
		return Outcome{}, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Outcome{}, fmt.Errorf("listen: %w", err)
	}
	host := ln.Addr().String()

	s := &server{
		req:     req,
		token:   token,
		host:    host,
		origin:  "http://" + host,
		tmpl:    tmpl,
		results: map[string]Result{},
		allowed: map[string]bool{},
		finish:  make(chan Status, 1),
	}
	for _, r := range req.Required {
		s.allowed[r.Name] = true
	}
	for _, e := range req.Existing {
		s.allowed[e.Name] = true
	}

	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)

	url := fmt.Sprintf("http://%s/?t=%s", host, token)
	if req.OnReady != nil {
		req.OnReady(url)
	}
	if req.OpenBrowser != nil {
		if err := req.OpenBrowser(url); err != nil {
			fmt.Fprintf(req.Log, "Could not open browser: %v\n", err)
		}
	}

	var status Status
	timer := time.NewTimer(req.Timeout)
	defer timer.Stop()
	select {
	case status = <-s.finish:
	case <-timer.C:
		status = StatusTimeout
	case <-ctx.Done():
		status = StatusCancelled
	}

	// handleSave may have completed everything just as the timer/ctx fired;
	// taking mu waits for any in-flight save, then the saved flag wins.
	if status != StatusSaved {
		s.mu.Lock()
		if s.saved {
			status = StatusSaved
		}
		s.mu.Unlock()
	}

	sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		srv.Close()
	}
	return Outcome{Status: status, Results: s.sortedResults()}, nil
}

func (s *server) sortedResults() []Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Result, 0, len(s.results))
	for _, r := range s.results {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// pendingLocked returns required names not yet saved. Caller holds s.mu.
func (s *server) pendingLocked() []string {
	p := []string{}
	for _, r := range s.req.Required {
		if !s.results[r.Name].Saved {
			p = append(p, r.Name)
		}
	}
	return p
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", csp)
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-Content-Type-Options", "nosniff")

	if r.Host != s.host || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.token)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	switch r.URL.Path {
	case "/":
		s.serveIndex(w, r)
	case "/app.js":
		s.serveAsset(w, "assets/app.js", "text/javascript; charset=utf-8")
	case "/app.css":
		s.serveAsset(w, "assets/app.css", "text/css; charset=utf-8")
	case "/api/state":
		s.handleState(w, r)
	case "/api/save", "/api/cancel":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Origin") != s.origin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/api/save" {
			s.handleSave(w, r)
		} else {
			s.handleCancel(w)
		}
	default:
		http.NotFound(w, r)
	}
}

func (s *server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var b strings.Builder
	if err := s.tmpl.Execute(&b, map[string]string{"Token": s.token}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, b.String())
}

func (s *server) serveAsset(w http.ResponseWriter, name, ctype string) {
	b, err := assets.ReadFile(name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Write(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type reqItem struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		Exists      bool       `json:"exists"`
		UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	}
	existing := map[string]Entry{}
	for _, e := range s.req.Existing {
		existing[e.Name] = e
	}
	required := make([]reqItem, 0, len(s.req.Required))
	for _, rs := range s.req.Required {
		it := reqItem{Name: rs.Name, Description: rs.Description}
		if e, ok := existing[rs.Name]; ok {
			it.Exists = e.Status == EntrySet
			it.UpdatedAt = e.UpdatedAt
		}
		required = append(required, it)
	}
	ex := s.req.Existing
	if ex == nil {
		ex = []Entry{}
	}
	s.mu.Lock()
	pending := s.pendingLocked()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"namespace":    s.req.Namespace,
		"scope":        s.req.Scope,
		"storage_note": s.req.StorageNote,
		"required":     required,
		"existing":     ex,
		"pending":      pending,
	})
}

func (s *server) handleSave(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body struct {
		Secrets []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"secrets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bad := func(msg string) { writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg}) }
	if len(body.Secrets) == 0 {
		bad("no secrets submitted")
		return
	}
	type item struct {
		name  string
		value []byte
	}
	items := make([]item, 0, len(body.Secrets))
	seen := map[string]bool{}
	for _, sv := range body.Secrets {
		if !s.allowed[sv.Name] {
			bad("name not allowed: " + sv.Name)
			return
		}
		if seen[sv.Name] {
			bad("duplicate name: " + sv.Name)
			return
		}
		seen[sv.Name] = true
		v := strings.TrimRight(sv.Value, "\r\n")
		if v == "" {
			bad("empty value for " + sv.Name)
			return
		}
		items = append(items, item{sv.Name, []byte(v)})
	}
	for _, p := range s.pendingLocked() {
		if !seen[p] {
			bad("missing required secret: " + p)
			return
		}
	}

	allOK := true
	results := make([]Result, 0, len(items))
	for _, it := range items {
		warning, err := s.req.Save(it.name, it.value)
		res := Result{Name: it.name, Saved: err == nil, Warning: warning}
		if err != nil {
			res.Error = strings.ReplaceAll(err.Error(), string(it.value), "***")
			allOK = false
		}
		s.results[it.name] = res
		results = append(results, res)
	}
	done := allOK && len(s.pendingLocked()) == 0
	if done {
		s.saved = true
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "done": done})
	if done {
		s.signal(StatusSaved)
	}
}

func (s *server) handleCancel(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]bool{"done": true})
	s.signal(StatusCancelled)
}

func (s *server) signal(st Status) {
	select {
	case s.finish <- st:
	default:
	}
}
