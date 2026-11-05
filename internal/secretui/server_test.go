package secretui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSave struct {
	mu       sync.Mutex
	calls    map[string][]string
	failOnce map[string]bool
	warn     map[string]string
}

func newFakeSave() *fakeSave {
	return &fakeSave{calls: map[string][]string{}, failOnce: map[string]bool{}, warn: map[string]string{}}
}

func (f *fakeSave) save(name string, value []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[name] = append(f.calls[name], string(value))
	if f.failOnce[name] {
		delete(f.failOnce, name)
		return "", errors.New("store boom")
	}
	return f.warn[name], nil
}

func (f *fakeSave) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += len(c)
	}
	return n
}

type env struct {
	base  string
	token string
	port  string
	done  <-chan Outcome
	log   *syncBuf
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func startRun(t *testing.T, req Request) *env {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	req.OnReady = func(u string) { ready <- u }
	lb := &syncBuf{}
	req.Log = lb
	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second
	}
	done := make(chan Outcome, 1)
	go func() {
		out, err := Run(ctx, req)
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		done <- out
	}()
	var raw string
	select {
	case raw = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("server not ready")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &env{base: "http://" + u.Host, token: u.Query().Get("t"), port: u.Port(), done: done, log: lb}
}

func (e *env) wait(t *testing.T) Outcome {
	t.Helper()
	select {
	case o := <-e.done:
		return o
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not finish")
		return Outcome{}
	}
}

func (e *env) notDone(t *testing.T) {
	t.Helper()
	select {
	case <-e.done:
		t.Fatal("Run finished unexpectedly")
	case <-time.After(100 * time.Millisecond):
	}
}

func (e *env) get(t *testing.T, path, token string) (*http.Response, string) {
	t.Helper()
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	resp, err := http.Get(e.base + path + sep + "t=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func (e *env) post(t *testing.T, path, origin, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest("POST", e.base+path+"?t="+e.token, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func (e *env) origin() string { return "http://127.0.0.1:" + e.port }

func (e *env) save(t *testing.T, kv ...string) (*http.Response, string) {
	t.Helper()
	type item struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	var items []item
	for i := 0; i+1 < len(kv); i += 2 {
		items = append(items, item{kv[i], kv[i+1]})
	}
	b, _ := json.Marshal(map[string]any{"secrets": items})
	return e.post(t, "/api/save", e.origin(), string(b))
}

func baseReq(f *fakeSave, required ...string) Request {
	var rs []RequiredSecret
	for _, n := range required {
		rs = append(rs, RequiredSecret{Name: n, Description: "desc " + n})
	}
	return Request{
		Namespace:   "ns1",
		Scope:       "global",
		StorageNote: "note",
		Required:    rs,
		Existing:    []Entry{{Name: "X", Status: EntrySet}},
		Save:        f.save,
	}
}

func TestRejectsWrongToken(t *testing.T) {
	e := startRun(t, baseReq(newFakeSave(), "A"))
	resp, _ := e.get(t, "/api/state", "bad")
	if resp.StatusCode != 403 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	resp, _ = e.get(t, "/api/state", e.token)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRejectsWrongHost(t *testing.T) {
	e := startRun(t, baseReq(newFakeSave(), "A"))
	req, _ := http.NewRequest("GET", e.base+"/api/state?t="+e.token, nil)
	req.Host = "localhost:" + e.port
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSaveRejectsMissingOrForeignOrigin(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A"))
	body := `{"secrets":[{"name":"A","value":"v"}]}`
	for _, origin := range []string{"", "http://evil.example"} {
		resp, _ := e.post(t, "/api/save", origin, body)
		if resp.StatusCode != 403 {
			t.Fatalf("origin %q: status %d", origin, resp.StatusCode)
		}
	}
	resp, _ := e.post(t, "/api/cancel", "http://evil.example", "{}")
	if resp.StatusCode != 403 {
		t.Fatalf("cancel status %d", resp.StatusCode)
	}
	if f.callCount() != 0 {
		t.Fatal("Save called")
	}
	e.notDone(t)
}

func TestNonPostIs405(t *testing.T) {
	e := startRun(t, baseReq(newFakeSave(), "A"))
	req, _ := http.NewRequest("GET", e.base+"/api/save?t="+e.token, nil)
	req.Header.Set("Origin", e.origin())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	e := startRun(t, baseReq(newFakeSave(), "A"))
	resp, body := e.get(t, "/", e.token)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !strings.Contains(body, "app.js?t="+e.token) {
		t.Fatalf("token not injected: %s", body)
	}
	want := map[string]string{
		"Content-Security-Policy": "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		"Cache-Control":           "no-store",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	for _, p := range []string{"/app.js", "/app.css"} {
		resp, _ := e.get(t, p, e.token)
		if resp.StatusCode != 200 || resp.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("%s: status %d csp %q", p, resp.StatusCode, resp.Header.Get("Content-Security-Policy"))
		}
	}
	resp, _ = e.get(t, "/nope", "bad")
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Error("403 response lacks CSP")
	}
}

func TestStateNeverContainsValues(t *testing.T) {
	f := newFakeSave()
	f.failOnce["B"] = true
	e := startRun(t, baseReq(f, "A", "B"))
	resp, body := e.save(t, "A", "s3cr3t-value", "B", "s3cr3t-value")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "s3cr3t-value") {
		t.Fatal("save response leaks value")
	}
	_, state := e.get(t, "/api/state", e.token)
	if strings.Contains(state, "s3cr3t-value") {
		t.Fatal("state leaks value")
	}
}

func TestStateIsRepeatableAndReflectsProgress(t *testing.T) {
	f := newFakeSave()
	f.failOnce["B"] = true
	e := startRun(t, baseReq(f, "A", "B"))
	e.save(t, "A", "1", "B", "2")
	for i := 0; i < 2; i++ {
		resp, body := e.get(t, "/api/state", e.token)
		if resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		var st struct {
			Namespace string   `json:"namespace"`
			Pending   []string `json:"pending"`
			Required  []struct {
				Name   string `json:"name"`
				Exists bool   `json:"exists"`
			} `json:"required"`
		}
		if err := json.Unmarshal([]byte(body), &st); err != nil {
			t.Fatal(err)
		}
		if len(st.Pending) != 1 || st.Pending[0] != "B" {
			t.Fatalf("pending = %v", st.Pending)
		}
		if st.Namespace != "ns1" || len(st.Required) != 2 {
			t.Fatalf("state = %s", body)
		}
	}
	e.notDone(t)
}

func TestStateExistsFromExisting(t *testing.T) {
	f := newFakeSave()
	ts := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	req := baseReq(f, "A")
	req.Existing = []Entry{{Name: "A", Status: EntrySet, UpdatedAt: &ts}}
	e := startRun(t, req)
	_, body := e.get(t, "/api/state", e.token)
	var st struct {
		Required []struct {
			Exists    bool       `json:"exists"`
			UpdatedAt *time.Time `json:"updated_at"`
		} `json:"required"`
	}
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Required[0].Exists || st.Required[0].UpdatedAt == nil {
		t.Fatalf("state = %s", body)
	}
}

func TestSaveRejectsNameNotAllowed(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A"))
	resp, _ := e.save(t, "A", "v", "Z", "v")
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if f.callCount() != 0 {
		t.Fatal("Save called")
	}
}

func TestSaveRejectsDuplicateAndEmpty(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A"))
	if resp, _ := e.save(t, "A", "v", "A", "w"); resp.StatusCode != 400 {
		t.Fatalf("dup status %d", resp.StatusCode)
	}
	if resp, _ := e.save(t, "A", "\r\n"); resp.StatusCode != 400 {
		t.Fatalf("empty status %d", resp.StatusCode)
	}
	if f.callCount() != 0 {
		t.Fatal("Save called")
	}
}

func TestSaveRequiresAllPending(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A", "B"))
	resp, _ := e.save(t, "A", "v")
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if f.callCount() != 0 {
		t.Fatal("Save called")
	}
}

func TestSaveTrimsTrailingNewlineOnly(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A"))
	resp, body := e.save(t, "A", " ab c \r\n")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d %s", resp.StatusCode, body)
	}
	e.wait(t)
	if got := f.calls["A"]; len(got) != 1 || got[0] != " ab c " {
		t.Fatalf("Save got %q", got)
	}
}

func TestSaveRejectsOversizedBody(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A"))
	big := `{"secrets":[{"name":"A","value":"` + strings.Repeat("x", 70*1024) + `"}]}`
	resp, _ := e.post(t, "/api/save", e.origin(), big)
	if resp.StatusCode != 413 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if f.callCount() != 0 {
		t.Fatal("Save called")
	}
}

func TestSaveAllSuccessFinishesSaved(t *testing.T) {
	f := newFakeSave()
	e := startRun(t, baseReq(f, "A"))
	resp, body := e.save(t, "X", "vx", "A", "va")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var r struct {
		Done bool `json:"done"`
	}
	json.Unmarshal([]byte(body), &r)
	if !r.Done {
		t.Fatalf("body %s", body)
	}
	out := e.wait(t)
	if out.Status != StatusSaved || len(out.Results) != 2 ||
		out.Results[0].Name != "A" || !out.Results[0].Saved ||
		out.Results[1].Name != "X" || !out.Results[1].Saved {
		t.Fatalf("outcome %+v", out)
	}
}

func TestPartialFailureAllowsRetry(t *testing.T) {
	f := newFakeSave()
	f.failOnce["B"] = true
	e := startRun(t, baseReq(f, "A", "B"))
	_, body := e.save(t, "A", "1", "B", "2")
	var r struct {
		Done    bool     `json:"done"`
		Results []Result `json:"results"`
	}
	json.Unmarshal([]byte(body), &r)
	if r.Done || len(r.Results) != 2 || r.Results[1].Error == "" {
		t.Fatalf("body %s", body)
	}
	e.notDone(t)
	resp, body := e.save(t, "B", "2")
	if resp.StatusCode != 200 || !strings.Contains(body, `"done":true`) {
		t.Fatalf("retry %d %s", resp.StatusCode, body)
	}
	out := e.wait(t)
	if out.Status != StatusSaved || len(out.Results) != 2 || !out.Results[0].Saved || !out.Results[1].Saved {
		t.Fatalf("outcome %+v", out)
	}
}

func TestWarningIsReported(t *testing.T) {
	f := newFakeSave()
	f.warn["A"] = "cfg"
	e := startRun(t, baseReq(f, "A"))
	e.save(t, "A", "v")
	out := e.wait(t)
	if out.Results[0].Warning != "cfg" {
		t.Fatalf("outcome %+v", out)
	}
}

func TestCancel(t *testing.T) {
	e := startRun(t, baseReq(newFakeSave(), "A"))
	resp, body := e.post(t, "/api/cancel", e.origin(), "{}")
	if resp.StatusCode != 200 || !strings.Contains(body, `"done":true`) {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	if out := e.wait(t); out.Status != StatusCancelled {
		t.Fatalf("status %s", out.Status)
	}
}

func TestContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan Outcome, 1)
	go func() {
		o, _ := Run(ctx, Request{Required: []RequiredSecret{{Name: "A"}}, Save: newFakeSave().save,
			Timeout: time.Minute, OnReady: func(u string) { ready <- u }})
		done <- o
	}()
	<-ready
	cancel()
	select {
	case o := <-done:
		if o.Status != StatusCancelled {
			t.Fatalf("status %s", o.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no finish")
	}
}

func TestTimeout(t *testing.T) {
	f := newFakeSave()
	f.failOnce["B"] = true
	req := baseReq(f, "A", "B")
	req.Timeout = 300 * time.Millisecond
	e := startRun(t, req)
	e.save(t, "A", "1", "B", "2")
	out := e.wait(t)
	if out.Status != StatusTimeout {
		t.Fatalf("status %s", out.Status)
	}
	if len(out.Results) != 2 || !out.Results[0].Saved || out.Results[1].Saved {
		t.Fatalf("results %+v", out.Results)
	}
}

func TestOpenBrowserFailureKeepsWaiting(t *testing.T) {
	f := newFakeSave()
	req := baseReq(f, "A")
	req.OpenBrowser = func(string) error { return errors.New("no browser") }
	e := startRun(t, req)
	time.Sleep(100 * time.Millisecond)
	if !strings.Contains(e.log.String(), "Could not open browser") {
		t.Fatalf("log = %q", e.log.String())
	}
	e.save(t, "A", "v")
	if out := e.wait(t); out.Status != StatusSaved {
		t.Fatalf("status %s", out.Status)
	}
}

func TestDoneWinsOverConcurrentTimeout(t *testing.T) {
	slow := func(name string, value []byte) (string, error) {
		time.Sleep(400 * time.Millisecond)
		return "", nil
	}
	req := baseReq(newFakeSave(), "A")
	req.Save = slow
	req.Timeout = 100 * time.Millisecond
	e := startRun(t, req)
	resp, body := e.save(t, "A", "v")
	if resp.StatusCode != 200 || !strings.Contains(body, `"done":true`) {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	if out := e.wait(t); out.Status != StatusSaved {
		t.Fatalf("status %s, want saved", out.Status)
	}
}

func TestSaveErrorContainingValueIsRedacted(t *testing.T) {
	req := baseReq(newFakeSave(), "A")
	req.Save = func(name string, value []byte) (string, error) {
		return "", errors.New("backend rejected " + string(value))
	}
	req.Timeout = 300 * time.Millisecond
	e := startRun(t, req)
	_, body := e.save(t, "A", "leaky-value-123")
	if strings.Contains(body, "leaky-value-123") {
		t.Fatalf("response leaks value: %s", body)
	}
	out := e.wait(t)
	for _, r := range out.Results {
		if strings.Contains(r.Error, "leaky-value-123") || r.Error == "" {
			t.Fatalf("result %+v", r)
		}
	}
}

func TestSaveErrorContainingValueIsWithheldWholesale(t *testing.T) {
	const withheld = "save failed (error message withheld because it contained the secret value)"
	req := baseReq(newFakeSave(), "A")
	req.Save = func(name string, value []byte) (string, error) {
		return "", errors.New("store boom")
	}
	req.Timeout = 300 * time.Millisecond
	e := startRun(t, req)
	_, body := e.save(t, "A", "boom")
	if strings.Contains(body, "boom") || strings.Contains(body, "***") || !strings.Contains(body, withheld) {
		t.Fatalf("response: %s", body)
	}
	out := e.wait(t)
	if len(out.Results) != 1 || out.Results[0].Error != withheld {
		t.Fatalf("outcome %+v", out)
	}
}
