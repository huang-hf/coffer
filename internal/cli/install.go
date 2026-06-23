package cli

import (
	"fmt"
	"io"

	"github.com/huang-hf/coffer/internal/skill"
)

func runInstallSkill(agent string, stdout io.Writer, stderr io.Writer) int {
	if skill.IsInstalled(agent) {
		fmt.Fprintf(stdout, "coffer skill already installed, updating...\n")
	}

	if err := skill.Install(agent); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "✓ coffer skill installed for %s\n", agent)
	fmt.Fprintf(stdout, "  Restart your agent to pick up the new skill.\n")
	return 0
}
