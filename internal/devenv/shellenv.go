package devenv

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Shellenv prints code for the caller to evaluate in its current zsh/bash shell.
func Shellenv(ctx context.Context, out io.Writer) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("mak's coding environment currently supports macOS only")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	i := &installer{env: os.Environ(), run: runner(strings.NewReader(""), io.Discard, nil)}
	return i.shellenv(ctx, out)
}

func (i *installer) shellenv(ctx context.Context, out io.Writer) error {
	shell := filepath.Base(envValue(i.env, "SHELL"))
	if shell != "." && shell != "zsh" && shell != "bash" {
		return fmt.Errorf("mak shellenv supports zsh and bash; current login shell is %q", shell)
	}
	if !i.findBrew() {
		return fmt.Errorf("Homebrew was not found; run mak setup dev first")
	}
	env := setEnv(i.env, "HOMEBREW_NO_AUTO_UPDATE", "1")
	env = setEnv(env, "HOMEBREW_NO_ANALYTICS", "1")
	prefix, err := i.run(ctx, env, i.brew, "--prefix")
	if err != nil {
		return err
	}
	prefix = strings.TrimSpace(prefix)
	if !filepath.IsAbs(prefix) || strings.ContainsAny(prefix, "\r\n:\x00") {
		return fmt.Errorf("invalid Homebrew prefix %q", prefix)
	}
	// Bash output is also valid zsh code, irrespective of Homebrew's shell detection.
	brewEnv, err := i.run(ctx, env, i.brew, "shellenv", "bash")
	if err != nil {
		return err
	}
	var tools, paths []string
	managed := make(map[string]bool)
	for _, rel := range []string{"opt/ruby/bin", "opt/python/libexec/bin", "bin", "sbin"} {
		path := filepath.Join(prefix, rel)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			tools = append(tools, path)
			managed[path] = true
		}
	}
	inserted := false
	for _, path := range strings.Split(envValue(i.env, "PATH"), string(os.PathListSeparator)) {
		if managed[path] {
			continue
		}
		// Keep user/project paths (venvs, nvm, rbenv, etc.) ahead of fallback
		// runtimes, while making Homebrew tools available before system tools.
		if !inserted && (path == "/usr/bin" || path == "/bin" || path == "/usr/sbin" || path == "/sbin" || path == "/usr/local/bin" || path == "/usr/local/sbin") {
			paths = append(paths, tools...)
			inserted = true
		}
		paths = append(paths, path)
	}
	if !inserted {
		paths = append(paths, tools...)
	}
	quotedPath := "'" + strings.ReplaceAll(strings.Join(paths, string(os.PathListSeparator)), "'", "'\\''") + "'"
	_, err = fmt.Fprintf(out, "%s\nexport PATH=%s\nhash -r\n", strings.TrimRight(brewEnv, "\n"), quotedPath)
	return err
}
