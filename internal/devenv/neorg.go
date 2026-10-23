package devenv

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SetupNeorg adds the bundled plugin spec without replacing an existing editor.
func SetupNeorg(ctx context.Context, in io.Reader, out io.Writer) error {
	i, err := packageInstaller(in, out)
	if err != nil {
		return err
	}
	if err := i.resolvePaths(); err != nil {
		return err
	}
	config := i.paths[0].path
	if _, err := os.Stat(filepath.Join(config, "lua", "config", "lazy.lua")); err != nil {
		return fmt.Errorf("Neorg setup requires an existing LazyVim configuration; run mak setup dev first")
	}
	if err := i.checkNeovim(ctx); err != nil {
		return err
	}
	for _, name := range []string{"luarocks", "terminal-notifier"} {
		if err := InstallPackage(ctx, in, out, name); err != nil {
			return err
		}
	}
	unlock, err := i.prepare(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err := i.checkNeovim(ctx); err != nil {
		return err
	}
	notes := filepath.Join(i.home, "neorg")
	if err := os.MkdirAll(filepath.Join(notes, "journal"), 0700); err != nil {
		return err
	}
	plugin := filepath.Join(config, "lua", "plugins", "neorg.lua")
	if _, err := os.Stat(plugin); os.IsNotExist(err) {
		contents, err := assets.ReadFile("assets/nvim/lua/plugins/neorg.lua")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(plugin), 0755); err != nil {
			return err
		}
		if err := atomicWrite(plugin, contents, 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// Install missing dependencies without updating the rest of the user's plugins.
	script, err := os.CreateTemp("", "mak-neorg-*.lua")
	if err != nil {
		return err
	}
	defer os.Remove(script.Name())
	contents, err := assets.ReadFile("assets/neorg-setup.lua")
	if err != nil {
		script.Close()
		return err
	}
	if _, err := script.Write(contents); err != nil {
		script.Close()
		return err
	}
	if err := script.Close(); err != nil {
		return err
	}
	i.env = setEnv(i.env, "MAK_NVIM_CONFIG", config)
	if _, err := i.run(ctx, i.env, "nvim", "--headless", "-u", "NONE", "-i", "NONE", "-S", script.Name()); err != nil {
		return fmt.Errorf("installing Neorg plugins: %w; existing editor configuration was retained", err)
	}
	fmt.Fprintf(out, "Neorg ready at %s. In Neovim: :Neorg journal today (or tomorrow).\n", notes)
	return nil
}
