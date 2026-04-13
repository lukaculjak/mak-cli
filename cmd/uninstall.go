package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall mak from your system",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Are you sure you want to uninstall mak? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			if strings.ToLower(strings.TrimSpace(input)) != "y" {
				fmt.Println("Uninstall cancelled.")
				return nil
			}

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("finding mak binary: %w", err)
			}

			if err := os.Remove(execPath); err != nil {
				return fmt.Errorf("removing mak (try with sudo): %w", err)
			}

			fmt.Println("mak has been uninstalled.")
			return nil
		},
	}
}
