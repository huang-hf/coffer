package secret

import (
	"testing"
	"time"
)

func TestBackendAndModTimeForFileStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("COFFER_USE_KEYCHAIN", "")
	s, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}
	if got := Backend(s); got != BackendFilePlain {
		t.Fatalf("Backend = %q, want file-plain", got)
	}
	if _, ok := ModTime(s, "ns1", "A"); ok {
		t.Fatal("ModTime ok for missing secret")
	}
	before := time.Now().Add(-time.Second)
	if err := s.Set("ns1", "A", []byte("v")); err != nil {
		t.Fatal(err)
	}
	mt, ok := ModTime(s, "ns1", "A")
	if !ok || mt.Before(before) {
		t.Fatalf("ModTime = %v, %v", mt, ok)
	}
}

func TestBackendEncryptedWhenAgeKeyExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COFFER_USE_KEYCHAIN", "")
	dir, _ := StoreDir()
	if err := EnsureAgeKey(dir); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}
	if got := Backend(s); got != BackendFileEncrypted {
		t.Fatalf("Backend = %q", got)
	}
	_ = s.Set("ns1", "A", []byte("v"))
	if _, ok := ModTime(s, "ns1", "A"); !ok {
		t.Fatal("ModTime not found for encrypted secret")
	}
}

type fakeStore struct{ Store }

func TestBackendOtherForUnknownStore(t *testing.T) {
	if Backend(fakeStore{}) != BackendOther {
		t.Fatal("want other")
	}
	if _, ok := ModTime(fakeStore{}, "n", "x"); ok {
		t.Fatal("want false")
	}
}
