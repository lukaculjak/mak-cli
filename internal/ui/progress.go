package ui

import (
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/term"
)

// Progress keeps a spinner below streamed logs without overwriting partial lines.
// Redirected output and terminals without styling receive ordinary stage lines.
type Progress struct {
	mu        sync.Mutex
	once      sync.Once
	out       io.Writer
	palette   palette
	text      string
	frame     int
	visible   bool
	lineStart bool
	paused    bool
	closed    bool
	stop      chan struct{}
	done      chan struct{}
}

func NewProgress(out io.Writer) *Progress {
	p := &Progress{out: out, palette: colors(out), lineStart: true}
	if p.palette.enabled {
		p.stop, p.done = make(chan struct{}), make(chan struct{})
		go func() {
			defer close(p.done)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-p.stop:
					return
				case <-ticker.C:
					p.redraw()
				}
			}
		}()
	}
	return p
}

// Fd preserves terminal/color detection for other ui helpers using this writer.
func (p *Progress) Fd() uintptr {
	if file, ok := p.out.(interface{ Fd() uintptr }); ok {
		return file.Fd()
	}
	return ^uintptr(0)
}

func (p *Progress) Stage(step, total int, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	if !p.lineStart {
		fmt.Fprintln(p.out)
	}
	p.text = fmt.Sprintf("Step %d/%d: %s", step, total, text)
	fmt.Fprintf(p.out, "%s %s\n", p.palette.paint(p.palette.orange, "[step]"), p.text)
	p.lineStart = true
	p.draw()
}

func (p *Progress) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	n, err := p.out.Write(data)
	if n > 0 {
		p.lineStart = data[n-1] == '\n'
	}
	p.draw()
	return n, err
}

// Pause allows an interactive subprocess to own the terminal until Resume.
func (p *Progress) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
	p.clear()
}

func (p *Progress) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = false
	p.draw()
}

// Close stops animation and clears its line before success or recovery output.
func (p *Progress) Close() {
	p.once.Do(func() {
		if p.stop != nil {
			close(p.stop)
			<-p.done
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		p.clear()
		if !p.lineStart && p.text != "" {
			fmt.Fprintln(p.out)
			p.lineStart = true
		}
		p.closed = true
	})
}

func (p *Progress) redraw() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	p.draw()
}

// clear and draw are called with mu held; only the transient spinner is erased.
func (p *Progress) clear() {
	if p.visible {
		fmt.Fprint(p.out, "\r\x1b[2K")
		p.visible = false
	}
}

func (p *Progress) draw() {
	if !p.palette.enabled || p.text == "" || !p.lineStart || p.paused || p.closed {
		return
	}
	frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
	line := []rune(string(frames[p.frame%len(frames)]) + " " + p.text)
	// Keep the spinner on one line, even in a narrow terminal.
	if width, _, err := term.GetSize(int(p.Fd())); err == nil && width > 1 && len(line) >= width {
		line = line[:width-1]
	}
	fmt.Fprint(p.out, p.palette.paint(p.palette.orange, string(line)))
	p.frame++
	p.visible = true
}
