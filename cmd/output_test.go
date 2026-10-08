package cmd

import (
	"bytes"
	"testing"

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

func TestMachineCommandsHaveNoHumanOutput(t *testing.T) {
	root := &cobra.Command{Use: "mak"}
	completion := &cobra.Command{Use: "completion"}
	bash := &cobra.Command{Use: "bash"}
	hidden := &cobra.Command{Use: "native-host", Hidden: true}
	regular := &cobra.Command{Use: "setup"}
	root.AddCommand(completion, hidden, regular)
	completion.AddCommand(bash)
	for _, cmd := range []*cobra.Command{completion, bash, hidden} {
		if humanOutput(cmd) {
			t.Fatalf("machine output would be decorated: %s", cmd.CommandPath())
		}
	}
	if !humanOutput(regular) {
		t.Fatal("ordinary command not decorated")
	}
}
