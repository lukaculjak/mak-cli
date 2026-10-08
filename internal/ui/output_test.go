package ui

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestHelpPreservesAlignmentAndText(t *testing.T) {
	original := "Description\n\nUsage:\n  mak setup [command]\n\nAvailable Commands:\n  dev         Install environment\n  validation  Generate composables\n\nFlags:\n  -h, --help   help for setup\n\nUse \"mak setup --help\" for details.\n"
	p := palette{enabled: true, orange: "1;38;2;255;149;0"}
	styled := p.help(original)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(styled, "")
	if plain != original {
		t.Fatalf("help layout/text changed:\n%q", plain)
	}
	for _, label := range []string{"Usage:", "Available Commands:", "dev", "validation", "Flags:", "-h, --help"} {
		if !strings.Contains(styled, p.paint(p.orange, label)) {
			t.Fatalf("label not accented: %s", label)
		}
	}
	if strings.Contains(styled, p.paint(p.orange, "Install environment")) {
		t.Fatal("description accented instead of command")
	}
}

func TestNonTerminalOutputContainsNoFormatting(t *testing.T) {
	var output bytes.Buffer
	Begin(&output, "mak setup")
	Step(&output, "Installing %s...", "tools")
	Success(&output, "Ready")
	Warning(&output, "Backup retained")
	Error(&output, "Failed")
	End(&output)
	want := "[step] Installing tools...\n[ok] Ready\n[warning] Backup retained\n[error] Failed\n"
	if output.String() != want {
		t.Fatalf("unexpected plain output: %q", output.String())
	}
}

func TestRedirectedHelpIsUnchanged(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	var output bytes.Buffer
	Help(&output, "mak setup", "Usage:\n  mak setup [command]\n")
	if output.String() != "Usage:\n  mak setup [command]\n" {
		t.Fatalf("redirected help changed: %q", output.String())
	}
}
