package cli

import (
	"reflect"
	"testing"
	"time"

	"github.com/huang-hf/coffer/internal/secret"
	"github.com/huang-hf/coffer/internal/secretui"
)

func TestParseSecretAddArgsDefaults(t *testing.T) {
	a, err := parseSecretAddArgs([]string{"A", "--ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !a.UI || !reflect.DeepEqual(a.Names, []string{"A"}) || a.Timeout != 5*time.Minute {
		t.Fatalf("got %+v", a)
	}
}

func TestParseSecretAddArgsNoUISingle(t *testing.T) {
	a, err := parseSecretAddArgs([]string{"A"})
	if err != nil || a.UI || len(a.Names) != 1 {
		t.Fatalf("got %+v, %v", a, err)
	}
}

func TestParseSecretAddArgsDedupes(t *testing.T) {
	a, err := parseSecretAddArgs([]string{"A", "B", "A", "--ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Names, []string{"A", "B"}) {
		t.Fatalf("names %v", a.Names)
	}
}

func TestParseSecretAddArgsDesc(t *testing.T) {
	a, err := parseSecretAddArgs([]string{"A", "B", "--ui", "--desc=A=用于 x=y", "--desc", "B=z", "--timeout", "30s"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "用于 x=y", "B": "z"}
	if !reflect.DeepEqual(a.Descs, want) {
		t.Fatalf("descs %v", a.Descs)
	}
	if a.Timeout != 30*time.Second {
		t.Fatalf("timeout %v", a.Timeout)
	}
}

func TestParseSecretAddArgsErrors(t *testing.T) {
	cases := [][]string{
		{"A", "B"},
		{"A", "--desc=A=x"},
		{"A", "--ui", "--desc=C=x"},
		{"A", "--ui", "--timeout=0s"},
		{"A", "--ui", "--timeout=abc"},
		{"A", "--timeout=1m"},
		{"bad name", "--ui"},
		{"--ui"},
		{"A", "--ui", "--bogus"},
		{"A", "--ui", "--desc=nodelim"},
	}
	for _, c := range cases {
		if _, err := parseSecretAddArgs(c); err == nil {
			t.Errorf("%v: expected error", c)
		}
	}
}

func TestBuildEntries(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	got := buildEntries([]string{"B", "C"}, []string{"A", "B"}, func(n string) (time.Time, bool) {
		if n == "A" {
			return ts, true
		}
		return time.Time{}, false
	})
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Name != "A" || got[0].Status != secretui.EntrySet || got[0].UpdatedAt == nil || !got[0].UpdatedAt.Equal(ts) {
		t.Errorf("A: %+v", got[0])
	}
	if got[1].Name != "B" || got[1].Status != secretui.EntrySet || got[1].UpdatedAt != nil {
		t.Errorf("B: %+v", got[1])
	}
	if got[2].Name != "C" || got[2].Status != secretui.EntryMissing || got[2].UpdatedAt != nil {
		t.Errorf("C: %+v", got[2])
	}
}

func TestStorageNoteCopy(t *testing.T) {
	cases := map[secret.BackendKind]string{
		secret.BackendFileEncrypted: "🔒 只在本机传输，加密写入 ~/.coffer，agent 看不到内容",
		secret.BackendFilePlain:     "🔒 只在本机传输，以仅本人可读的文件写入 ~/.coffer（未加密），agent 看不到内容",
		secret.BackendOSKeychain:    "🔒 只在本机传输，写入 macOS 钥匙串，agent 看不到内容",
		secret.BackendOther:         "🔒 只在本机传输，写入系统凭据存储，agent 看不到内容",
	}
	for k, want := range cases {
		if got := storageNote(k); got != want {
			t.Errorf("%s: %q", k, got)
		}
	}
}
