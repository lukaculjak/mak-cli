// Package ui provides terminal styling for mak's human-facing output.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type palette struct {
	enabled bool
	orange  string
}

func colors(w io.Writer) palette {
	file, ok := w.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(int(file.Fd())) || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return palette{}
	}
	terminal, color := os.Getenv("TERM"), os.Getenv("COLORTERM")
	orange := "1;33" // Basic ANSI terminals fall back to warm yellow.
	if color == "truecolor" || color == "24bit" || strings.Contains(terminal, "ghostty") || strings.Contains(terminal, "direct") {
		orange = "1;38;2;255;149;0"
	} else if strings.Contains(terminal, "256color") {
		orange = "1;38;5;208"
	}
	return palette{enabled: true, orange: orange}
}

func (p palette) paint(code, text string) string {
	if !p.enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// Begin/End add visual boundaries on terminals without changing piped output.
func Begin(w io.Writer, command string) {
	p := colors(w)
	if p.enabled {
		fmt.Fprintf(w, "\n%s\n\n", p.paint(p.orange, "── "+command+" ──"))
	}
}

func End(w io.Writer) {
	p := colors(w)
	if p.enabled {
		fmt.Fprintf(w, "\n%s\n\n", p.paint("1;32", "[ok] Done"))
	}
}

func Step(w io.Writer, format string, args ...any)    { message(w, "step", format, args...) }
func Success(w io.Writer, format string, args ...any) { message(w, "ok", format, args...) }
func Warning(w io.Writer, format string, args ...any) { message(w, "warning", format, args...) }
func Error(w io.Writer, format string, args ...any)   { message(w, "error", format, args...) }

func Heading(w io.Writer, format string, args ...any) {
	p := colors(w)
	fmt.Fprintln(w, p.paint(p.orange, fmt.Sprintf(format, args...)))
}

// Check prints an installed/missing indicator, coloring only the checkbox.
func Check(w io.Writer, installed bool, format string, args ...any) {
	p := colors(w)
	label, code := "[ ]", "1;31"
	if installed {
		label, code = "[x]", "1;32"
	}
	fmt.Fprintf(w, "%s %s\n", p.paint(code, label), fmt.Sprintf(format, args...))
}

func message(w io.Writer, label, format string, args ...any) {
	p := colors(w)
	code := p.orange
	switch label {
	case "ok":
		code = "1;32"
	case "warning":
		code = p.orange
	case "error":
		code = "1;31"
	}
	fmt.Fprintf(w, "%s %s\n", p.paint(code, "["+label+"]"), fmt.Sprintf(format, args...))
}

// Prompt accents only the question, resetting color before the user's input.
func Prompt(format string, args ...any) {
	p := colors(os.Stdout)
	fmt.Fprint(os.Stdout, p.paint(p.orange, fmt.Sprintf(format, args...)))
}

// Help styles Cobra's already-aligned text, preserving flag and command padding.
func Help(w io.Writer, command, text string) {
	p := colors(w)
	if !p.enabled {
		fmt.Fprint(w, text)
		return
	}
	Begin(w, command)
	fmt.Fprintln(w, p.help(text))
}

func (p palette) help(text string) string {
	lines := strings.Split(text, "\n")
	section := ""
	for n, line := range lines {
		if line != "" && !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":") {
			section = line
			lines[n] = p.paint(p.orange, line)
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		indent := line[:len(line)-len(trimmed)]
		switch section {
		case "Usage:":
			if indent != "" && trimmed != "" {
				lines[n] = indent + p.paint(p.orange, trimmed)
			}
		case "Available Commands:", "Additional help topics:":
			if fields := strings.Fields(trimmed); indent != "" && len(fields) > 0 {
				name := fields[0]
				lines[n] = indent + p.paint(p.orange, name) + trimmed[len(name):]
			}
		case "Flags:", "Global Flags:":
			if strings.HasPrefix(trimmed, "-") {
				if end := strings.Index(trimmed, "  "); end >= 0 {
					lines[n] = indent + p.paint(p.orange, trimmed[:end]) + trimmed[end:]
				}
			}
		}
		if strings.HasPrefix(line, "Use \"") {
			lines[n] = p.paint("2", line)
		}
	}
	return strings.Join(lines, "\n")
}
