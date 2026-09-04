package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukaculjak/mak-cli/cmd/prefill"
	"github.com/lukaculjak/mak-cli/internal/meetings"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var purge bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall mak from your system",
		Args:  cobra.NoArgs,
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

			if purge {
				if err := purgeUserData(); err != nil {
					return fmt.Errorf("removing mak data: %w", err)
				}
			}

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("finding mak binary: %w", err)
			}

			if err := os.Remove(execPath); err != nil {
				return fmt.Errorf("removing mak (try with sudo): %w", err)
			}

			fmt.Println("mak has been uninstalled.")
			if purge {
				fmt.Println("Configuration, credentials, browser manifests, and cron jobs were removed.")
			} else {
				fmt.Println("Your configuration and credentials were preserved. Use --purge to remove them during uninstall.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false, "also remove configuration, credentials, browser manifests, and meeting cron jobs")
	return cmd
}

func purgeUserData() error {
	var errs []error
	if err := meetings.RemoveAllCronJobs(); err != nil {
		errs = append(errs, err)
	}
	if err := prefill.RemoveHostManifests(); err != nil {
		errs = append(errs, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		errs = append(errs, err)
	} else {
		configDir := filepath.Join(home, ".config", "mak")
		if err := os.RemoveAll(configDir); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
