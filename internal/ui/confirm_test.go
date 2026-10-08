package ui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestConfirmAnswersAndDefault(t *testing.T) {
	for _, tc := range []struct {
		input string
		yes   bool
	}{
		{"\n", true}, {"yes\n", true}, {" Y \r\n", true}, {"yes", true},
		{"n\n", false}, {"NO\n", false}, {"maybe\ny\n", true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var out bytes.Buffer
			yes, err := Confirm(context.Background(), strings.NewReader(tc.input), &out, "Install?")
			if err != nil || yes != tc.yes {
				t.Fatalf("answer %v, error %v", yes, err)
			}
			if !strings.Contains(out.String(), "Install? [Y/n]: ") || strings.Contains(out.String(), "\x1b") {
				t.Fatalf("wrong plain prompt: %q", out.String())
			}
		})
	}
}

func TestConfirmDoesNotAssumeYesWithoutInput(t *testing.T) {
	yes, err := Confirm(context.Background(), strings.NewReader(""), io.Discard, "Install?")
	if yes || err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("missing input accepted: %v %v", yes, err)
	}
}

func TestConfirmLeavesSubsequentInputAvailable(t *testing.T) {
	in := strings.NewReader("\nsubsequent input\n")
	yes, err := Confirm(context.Background(), in, io.Discard, "Install?")
	remaining, _ := io.ReadAll(in)
	if err != nil || !yes || string(remaining) != "subsequent input\n" {
		t.Fatalf("stdin consumed past confirmation: %v %v %q", yes, err, remaining)
	}
}

func TestConfirmCancellation(t *testing.T) {
	in, writer := io.Pipe()
	defer in.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	yes, err := Confirm(ctx, in, io.Discard, "Install?")
	if yes || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v %v", yes, err)
	}
}

func TestPlainCheckboxes(t *testing.T) {
	var out bytes.Buffer
	Check(&out, true, "%s (tracked by mak)", "neovim")
	Check(&out, false, "%s", "node")
	if out.String() != "[x] neovim (tracked by mak)\n[ ] node\n" {
		t.Fatalf("checkbox output: %q", out.String())
	}
}
