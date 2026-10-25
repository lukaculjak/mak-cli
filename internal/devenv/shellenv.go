package devenv

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/semver"
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
	quotedPath := "'" + strings.ReplaceAll(brewFallbackPath(prefix, envValue(i.env, "PATH")), "'", "'\\''") + "'"
	_, err = fmt.Fprintf(out, "%s\nexport PATH=%s\nhash -r\n", strings.TrimRight(brewEnv, "\n"), quotedPath)
	return err
}

func brewFallbackPath(prefix, original string) string {
	var tools, paths []string
	managed := make(map[string]bool)
	for _, path := range brewToolDirs(prefix) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			tools = append(tools, path)
			managed[path] = true
		}
	}
	inserted := false
	for _, path := range strings.Split(original, string(os.PathListSeparator)) {
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
	return strings.Join(paths, string(os.PathListSeparator))
}

func brewToolDirs(prefix string) []string {
	paths := []string{filepath.Join(prefix, "opt/ruby/bin"), filepath.Join(prefix, "opt/python/bin"), filepath.Join(prefix, "opt/python/libexec/bin")}
	// A versioned Homebrew Python can provide python3 without being linked into
	// brew's bin directory. Prefer the newest installed version as a fallback.
	versioned, _ := filepath.Glob(filepath.Join(prefix, "opt/python@*/bin"))
	version := func(path string) string {
		return "v" + strings.TrimPrefix(filepath.Base(filepath.Dir(path)), "python@")
	}
	slices.SortFunc(versioned, func(a, b string) int { return semver.Compare(version(b), version(a)) })
	for _, path := range versioned {
		if semver.IsValid(version(path)) && executable(path, "python3") != "" {
			paths = append(paths, path)
		}
	}
	return append(paths, filepath.Join(prefix, "bin"), filepath.Join(prefix, "sbin"))
}
