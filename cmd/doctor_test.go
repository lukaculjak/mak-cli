package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/spf13/cobra"
)

func TestDoctorVersionLastAndAlias(t *testing.T) {
	for _, name := range []string{"doctor", "healthcheck"} {
		for _, failure := range []error{nil, devenv.ErrUnhealthy, context.Canceled} {
			t.Run(fmt.Sprint(name, failure), func(t *testing.T) {
				var out bytes.Buffer
				calls := 0
				root := &cobra.Command{Use: "mak", SilenceErrors: true, SilenceUsage: true,
					PersistentPostRunE: func(*cobra.Command, []string) error { t.Fatal("doctor ran update/footer hook"); return nil }}
				root.AddCommand(newDoctorCmd(func(context.Context, io.Writer) error {
					calls++
					fmt.Fprintln(&out, "diagnostic findings")
					return failure
				}))
				root.SetOut(&out)
				root.SetErr(&out)
				root.SetArgs([]string{name})
				err := root.Execute()
				if calls != 1 || (failure != nil && (!errors.Is(err, failure) || !errors.Is(err, errDoctorReported))) || (failure == nil && err != nil) {
					t.Fatalf("calls=%d error=%v", calls, err)
				}
				if !strings.HasSuffix(out.String(), "\nmak version: "+Version+"\n") {
					t.Fatalf("version not last: %s", out.String())
				}
			})
		}
	}
}

func TestDoctorRejectsArguments(t *testing.T) {
	cmd := newDoctorCmd(func(context.Context, io.Writer) error { t.Fatal("unexpected checks"); return nil })
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"fix"})
	if cmd.Execute() == nil {
		t.Fatal("accepted an argument")
	}
}
