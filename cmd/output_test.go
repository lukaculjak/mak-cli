package cmd

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/lukaculjak/mak-cli/cmd/setup"
	"github.com/spf13/cobra"
)

func TestHelpRetainsDefaultCobraOutputWhenRedirected(t *testing.T) {
	root := &cobra.Command{Use: "mak"}
	child := setup.NewSetupCmd()
	root.AddCommand(child)
	var before, after bytes.Buffer
	child.SetOut(&before)
	child.Help()
	configureHelp(root)
	child.SetOut(&after)
	child.Help()
	if after.String() != before.String() {
		t.Fatalf("plain help changed:\n%s", after.String())
	}
}

func TestHumanCommandStartsANewElapsedTimer(t *testing.T) {
	cmd := &cobra.Command{Use: "setup"}
	cmd.SetOut(&bytes.Buffer{})
	previous := time.Now().Add(-2 * time.Hour)
	cmd.SetContext(context.WithValue(context.Background(), commandStartedKey{}, previous))
	rootCmd.PersistentPreRun(cmd, nil)
	started, ok := cmd.Context().Value(commandStartedKey{}).(time.Time)
	if !ok || !started.After(previous) {
		t.Fatal("command reused an old timer")
	}
	shell := newShellenvCmd()
	shell.SetContext(context.Background())
	rootCmd.PersistentPreRun(shell, nil)
	if shell.Context().Value(commandStartedKey{}) != nil {
		t.Fatal("machine-readable command started a footer timer")
	}
}

func TestMachineCommandsHaveNoHumanOutput(t *testing.T) {
	root := &cobra.Command{Use: "mak"}
	completion := &cobra.Command{Use: "completion"}
	shellenv := newShellenvCmd()
	bash := &cobra.Command{Use: "bash"}
	hidden := &cobra.Command{Use: "native-host", Hidden: true}
	regular := &cobra.Command{Use: "setup"}
	root.AddCommand(completion, shellenv, hidden, regular)
	completion.AddCommand(bash)
	for _, cmd := range []*cobra.Command{completion, bash, shellenv, hidden} {
		if humanOutput(cmd) {
			t.Fatalf("machine output would be decorated: %s", cmd.CommandPath())
		}
	}
	if !humanOutput(regular) {
		t.Fatal("ordinary command not decorated")
	}
}
