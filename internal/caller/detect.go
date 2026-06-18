package caller

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const (
	EnvMarker    = "COFFER_CALLER"
	EnvMarkerVal = "1"
)

func IsAuthorizedCaller() bool {
	if os.Getenv(EnvMarker) == EnvMarkerVal {
		return true
	}

	return checkProcessTree()
}

func checkProcessTree() bool {
	if runtime.GOOS == "windows" {
		return checkWindowsProcessTree()
	}
	return checkUnixProcessTree()
}

func checkUnixProcessTree() bool {
	ppid := os.Getppid()
	if ppid == 1 {
		return false
	}

	return validateParentProcess(ppid)
}

func checkWindowsProcessTree() bool {
	ppid := os.Getppid()
	return validateParentProcess(ppid)
}

func validateParentProcess(ppid int) bool {
	parentName := getProcessName(ppid)
	if parentName == "" {
		return false
	}

	authorizedPrefixes := []string{
		"coffer",
		"safehouse",
		"claude",
		"node",
		"python",
		"ruby",
		"go",
	}

	for _, prefix := range authorizedPrefixes {
		if strings.HasPrefix(strings.ToLower(parentName), prefix) {
			return true
		}
	}

	return false
}

func getProcessName(pid int) string {
	if runtime.GOOS == "windows" {
		return getWindowsProcessName(pid)
	}
	return getUnixProcessName(pid)
}

func getUnixProcessName(pid int) string {
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func getWindowsProcessName(pid int) string {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

func MarkAsCaller() {
	os.Setenv(EnvMarker, EnvMarkerVal)
}

func UnmarkAsCaller() {
	os.Unsetenv(EnvMarker)
}
