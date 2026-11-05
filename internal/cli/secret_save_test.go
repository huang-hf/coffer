package cli

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/huang-hf/coffer/internal/config"
)

type memStore struct {
	m       map[string][]byte
	failSet bool
}

func (s *memStore) Set(ns, name string, v []byte) error {
	if s.failSet {
		return errors.New("set failed")
	}
	s.m[ns+"/"+name] = v
	return nil
}
func (s *memStore) Get(ns, name string) ([]byte, error) {
	v, ok := s.m[ns+"/"+name]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}
func (s *memStore) Delete(ns, name string) error { delete(s.m, ns+"/"+name); return nil }
func (s *memStore) List(ns string) ([]string, error) {
	var out []string
	for k := range s.m {
		if strings.HasPrefix(k, ns+"/") {
			out = append(out, strings.TrimPrefix(k, ns+"/"))
		}
	}
	sort.Strings(out)
	return out, nil
}

func newTestCfg() *config.Config {
	return &config.Config{Secrets: map[string]string{}, Namespaces: map[string]*config.NamespaceConfig{}}
}

func TestSaveSecretValueAddsConfigEntryForNewName(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), ".coffer")
	st := &memStore{m: map[string][]byte{}}
	warn, err := saveSecretValue(st, newTestCfg(), cfgPath, "dev", "A", []byte("v"))
	if err != nil || warn != "" {
		t.Fatalf("err=%v warn=%q", err, warn)
	}
	if string(st.m["dev/A"]) != "v" {
		t.Fatalf("store = %v", st.m)
	}
	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.GetSecretsForNamespace("dev")["A"]; !ok {
		t.Fatal("config missing A")
	}
}

func TestSaveSecretValueKeepsExistingConfigValue(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), ".coffer")
	cfg := newTestCfg()
	cfg.Namespaces["dev"] = &config.NamespaceConfig{Secrets: map[string]string{"A": "{{coffer:A}}"}}
	st := &memStore{m: map[string][]byte{}}
	if _, err := saveSecretValue(st, cfg, cfgPath, "dev", "A", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetSecretsForNamespace("dev")["A"]; got != "{{coffer:A}}" {
		t.Fatalf("in-memory value = %q", got)
	}
	if _, err := os.Stat(cfgPath); err == nil {
		loaded, err := config.Load(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.GetSecretsForNamespace("dev")["A"]; got != "{{coffer:A}}" {
			t.Fatalf("reloaded value = %q", got)
		}
	}
}

func TestSaveSecretValueStoreFailure(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), ".coffer")
	st := &memStore{m: map[string][]byte{}, failSet: true}
	if _, err := saveSecretValue(st, newTestCfg(), cfgPath, "dev", "A", []byte("v")); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(cfgPath); err == nil {
		t.Fatal("config must not be written")
	}
}

func TestSaveSecretValueConfigFailureIsWarning(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "nodir", ".coffer")
	st := &memStore{m: map[string][]byte{}}
	warn, err := saveSecretValue(st, newTestCfg(), cfgPath, "dev", "A", []byte("v"))
	if err != nil || warn == "" {
		t.Fatalf("err=%v warn=%q", err, warn)
	}
	if string(st.m["dev/A"]) != "v" {
		t.Fatal("value not stored")
	}
}
