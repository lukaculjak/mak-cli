package prefill

import (
	"fmt"
	"os"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewPrefillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prefill",
		Short: "Manage login credential prefills for your projects",
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newNativeHostCmd())

	return cmd
}

// requireMasterPassword prompts for the master password, initializing the store
// on first use. Returns the verified password or an error.
func requireMasterPassword() (string, error) {
	if !prefills.HasStore() {
		return setupMasterPassword()
	}
	return promptMasterPassword("Master password: ")
}

// setupMasterPassword guides the user through setting a master password for the first time.
func setupMasterPassword() (string, error) {
	fmt.Println("Welcome to mak prefill!")
	fmt.Println("No store found. Let's set up a master password to protect your credentials.")
	fmt.Println()

	for {
		pw, err := promptMasterPassword("Choose a master password: ")
		if err != nil {
			return "", err
		}
		confirm, err := promptMasterPassword("Confirm master password: ")
		if err != nil {
			return "", err
		}
		if pw != confirm {
			fmt.Println("  Passwords do not match. Try again.")
			continue
		}
		if len(pw) < 8 {
			fmt.Println("  Password must be at least 8 characters.")
			continue
		}
		if err := prefills.InitStore(pw); err != nil {
			return "", fmt.Errorf("failed to initialize store: %w", err)
		}
		fmt.Println()
		fmt.Println("  Master password set. Store initialized.")
		fmt.Println()
		return pw, nil
	}
}

// promptMasterPassword reads a password from the terminal without echo.
func promptMasterPassword(label string) (string, error) {
	fmt.Print(label)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	if len(pw) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}
	return string(pw), nil
}

// verifyAndLoad prompts for the master password, verifies it, and loads projects.
func verifyAndLoad() (string, []prefills.Project, error) {
	pw, err := promptMasterPassword("Master password: ")
	if err != nil {
		return "", nil, err
	}
	ok, err := prefills.VerifyPassword(pw)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		return "", nil, fmt.Errorf("wrong master password")
	}
	projects, err := prefills.Load(pw)
	if err != nil {
		return "", nil, err
	}
	return pw, projects, nil
}
