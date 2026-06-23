package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var skillContent string

// InstallDir returns the target directory for a given agent skill system.
func InstallDir(agent string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}

	switch agent {
	case "claude-code":
		return filepath.Join(home, ".claude", "skills", "coffer"), nil
	case "codex":
		return filepath.Join(home, ".agents", "skills", "coffer"), nil
	default:
		return "", fmt.Errorf("unknown agent: %s (supported: claude-code, codex)", agent)
	}
}

// Install writes the embedded SKILL.md to the agent's skills directory.
// It is idempotent — running it again updates the installed skill.
func Install(agent string) error {
	dir, err := InstallDir(agent)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create skill directory %s: %w", dir, err)
	}

	target := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(target, []byte(skillContent), 0644); err != nil {
		return fmt.Errorf("cannot write skill file %s: %w", target, err)
	}

	return nil
}

// IsInstalled checks whether coffer skill is already installed for the given agent.
func IsInstalled(agent string) bool {
	dir, err := InstallDir(agent)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil
}
