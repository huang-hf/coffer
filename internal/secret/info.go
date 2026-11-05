package secret

import "time"

// BackendKind describes where a Store keeps its secrets.
type BackendKind string

const (
	BackendFileEncrypted BackendKind = "file-encrypted"
	BackendFilePlain     BackendKind = "file-plain"
	BackendOSKeychain    BackendKind = "os-keychain" // macOS Keychain
	BackendOther         BackendKind = "other"
)

type backendKinder interface {
	backendKind() BackendKind
}

type modTimer interface {
	modTime(namespace, name string) (time.Time, bool)
}

// Backend reports the backend kind of s; stores without the optional
// interface yield BackendOther.
func Backend(s Store) BackendKind {
	if b, ok := s.(backendKinder); ok {
		return b.backendKind()
	}
	return BackendOther
}

// ModTime returns when the secret was last written, if the store can tell.
func ModTime(s Store, namespace, name string) (time.Time, bool) {
	if m, ok := s.(modTimer); ok {
		return m.modTime(namespace, name)
	}
	return time.Time{}, false
}
