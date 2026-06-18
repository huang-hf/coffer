package caller

import (
	"os"
	"testing"
)

func TestIsAuthorizedCaller_EnvMarker(t *testing.T) {
	os.Setenv(EnvMarker, EnvMarkerVal)
	defer os.Unsetenv(EnvMarker)

	if !IsAuthorizedCaller() {
		t.Error("IsAuthorizedCaller() should return true when env marker is set")
	}
}

func TestIsAuthorizedCaller_NoEnvMarker(t *testing.T) {
	os.Unsetenv(EnvMarker)

	result := IsAuthorizedCaller()
	if result {
		t.Log("IsAuthorizedCaller() returned true via process tree (expected when run under go/claude)")
	} else {
		t.Log("IsAuthorizedCaller() returned false - parent not in authorized list")
	}
}

func TestMarkAsCaller(t *testing.T) {
	os.Unsetenv(EnvMarker)

	MarkAsCaller()

	if value := os.Getenv(EnvMarker); value != EnvMarkerVal {
		t.Errorf("MarkAsCaller() env %s = %v, want %v", EnvMarker, value, EnvMarkerVal)
	}

	UnmarkAsCaller()

	if value := os.Getenv(EnvMarker); value != "" {
		t.Errorf("UnmarkAsCaller() env %s = %v, want empty", EnvMarker, value)
	}
}

func TestCheckProcessTree(t *testing.T) {
	result := checkProcessTree()
	if result {
		t.Log("Process tree check returned true - parent process is authorized")
	} else {
		t.Log("Process tree check returned false - parent process is not authorized")
	}
}
