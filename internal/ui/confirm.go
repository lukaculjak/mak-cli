package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Confirm defaults to yes on Enter, but never treats missing input as consent.
func Confirm(ctx context.Context, in io.Reader, out io.Writer, question string) (bool, error) {
	return confirm(ctx, in, out, question, true)
}

// ConfirmOptIn requires an explicit yes; Enter and missing input decline.
func ConfirmOptIn(ctx context.Context, in io.Reader, out io.Writer, question string) (bool, error) {
	return confirm(ctx, in, out, question, false)
}

func confirm(ctx context.Context, in io.Reader, out io.Writer, question string, defaultYes bool) (bool, error) {
	choices, defaultText := " [y/N]: ", "no"
	if defaultYes {
		choices, defaultText = " [Y/n]: ", "yes"
	}
	for {
		p := colors(out)
		fmt.Fprint(out, p.paint(p.orange, question+choices))
		type reply struct {
			text string
			err  error
		}
		result := make(chan reply, 1)
		go func() {
			// Read only this line, leaving subsequent stdin for Homebrew/sudo.
			var line strings.Builder
			var b [1]byte
			for {
				_, err := io.ReadFull(in, b[:])
				if err != nil || b[0] == '\n' {
					result <- reply{line.String(), err}
					return
				}
				line.WriteByte(b[0])
			}
		}()
		var answer reply
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case answer = <-result:
		}
		if answer.err != nil && !errors.Is(answer.err, io.EOF) {
			return false, answer.err
		}
		text := strings.ToLower(strings.TrimSpace(answer.text))
		if text == "" && answer.err != nil {
			if !defaultYes {
				return false, nil
			}
			return false, fmt.Errorf("confirmation requires input; use --yes to confirm installation without a prompt")
		}
		switch text {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintf(out, "Please enter yes or no (Enter selects %s).\n", defaultText)
		}
	}
}
