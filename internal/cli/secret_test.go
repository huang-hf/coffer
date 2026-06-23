package cli

import (
	"io"
	"os"
	"testing"
)

func TestReadPasswordAcceptsNonTTYInputWithoutTrailingNewline(t *testing.T) {
	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer func() { os.Stdin = originalStdin }()
	os.Stdin = reader

	if _, err := writer.WriteString("secret-value"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	value, err := readPassword(io.Discard)
	if err != nil {
		t.Fatalf("readPassword() error = %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("readPassword() = %q, want %q", value, "secret-value")
	}
}

func TestReadPasswordTrimsTrailingNewline(t *testing.T) {
	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer func() { os.Stdin = originalStdin }()
	os.Stdin = reader

	if _, err := writer.WriteString("secret-value\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	value, err := readPassword(io.Discard)
	if err != nil {
		t.Fatalf("readPassword() error = %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("readPassword() = %q, want %q", value, "secret-value")
	}
}
