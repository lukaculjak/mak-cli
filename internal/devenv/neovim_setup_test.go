package devenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNeovimSetupOwnsMasonInstallLifecycle(t *testing.T) {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("Neovim is unavailable; the macOS smoke test exercises real setup")
	}
	root := t.TempDir()
	config := filepath.Join(root, "config", "nvim")
	lazy := filepath.Join(root, "data", "nvim", "lazy", "lazy.nvim", "lua")
	for _, dir := range []string{config, lazy} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		filepath.Join(config, "lazy-lock.json"): []byte("{}"),
		filepath.Join(config, "init.lua"): []byte(`
vim.opt.rtp:prepend(vim.fn.stdpath("data") .. "/lazy/lazy.nvim")
require("lazy").setup({ spec = {}, install = { colorscheme = { "habamax" } } })
`),
		filepath.Join(lazy, "lazy.lua"): []byte(`
local installed, installing, calls = false, false, 0
local tool = {
  is_installed = function() return installed end,
  is_installing = function() return installing end,
  install = function()
    assert(vim.env.MAK_NVIM_PHASE == "tools", "Mason installed a package outside the tools phase")
    assert(not installing, "Package is already installing.")
    calls, installing = calls + 1, true
    vim.defer_fn(function() installed, installing = true, false end, 20)
  end,
}
package.preload["mason"] = function() return { setup = function(opts)
  assert(opts.ensure_installed[1] == "debugpy", "Mason tool options were lost")
end } end
package.preload["mason-registry"] = function() return {
  refresh = function(callback) assert(vim.env.MAK_NVIM_PHASE == "tools"); callback(true) end,
  get_package = function() return tool end,
} end
package.preload["mason-lspconfig.mappings"] = function() return { get_mason_map = function()
  return { lspconfig_to_package = { pyright = "debugpy" } }
end } end
package.preload["nvim-treesitter"] = function() return {
  setup = function() end,
  install = function()
    assert(installed and not installing and calls == 1, "Setup did not wait for a single Mason installation")
    return { wait = function() return true end }
  end,
} end
local plugins = {
  ["mason.nvim"] = { "mason-org/mason.nvim", build = ":MasonUpdate", opts = { ensure_installed = { "debugpy" } },
    config = function() error("LazyVim's automatic Mason installation raced mak setup") end },
  ["nvim-treesitter"] = { "nvim-treesitter/nvim-treesitter", opts = { ensure_installed = {} },
    config = function() error("Syntax parsers installed before the tools phase") end },
  ["nvim-lspconfig"] = { opts = { servers = { pyright = {} } } },
}
package.preload["lazy.core.config"] = function() return { plugins = {} } end
package.preload["lazy.core.plugin"] = function() return { values = function(p) return plugins[p].opts end } end
-- The production script accesses plugin options via the resolved configuration.
local resolved = require("lazy.core.config").plugins
for name in pairs(plugins) do resolved[name] = name end
local function load_plugins()
  for _, name in ipairs({ "mason.nvim", "nvim-treesitter" }) do
    local plugin = plugins[name]
    if not plugin.loaded then
      assert(plugin.build ~= ":MasonUpdate", "MasonUpdate must not run during mak's plugin installation")
      plugin.config(plugin, plugin.opts)
      plugin.loaded = true
    end
  end
end
return {
  setup = function(settings)
    for _, spec in ipairs(settings.spec) do
      for name, plugin in pairs(plugins) do
        if spec[1] == plugin[1] then plugins[name] = vim.tbl_extend("force", plugin, spec) end
      end
    end
    assert(settings.install.colorscheme[1] == "habamax", "Other lazy settings were lost")
    if settings.install.missing ~= false then load_plugins() end
  end,
  install = function() load_plugins(); for name in pairs(resolved) do resolved[name] = nil end end,
  restore = function() end,
  load = function() load_plugins() end,
}
`),
	}
	files[filepath.Join(root, "setup.lua")], err = assets.ReadFile("assets/setup.lua")
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, phase := range []string{"plugins", "tools"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, nvim, "--headless", "-u", "NONE", "-i", "NONE", "-S", filepath.Join(root, "setup.lua"))
			cmd.Env = os.Environ()
			for key, value := range map[string]string{
				"HOME": root, "XDG_CONFIG_HOME": filepath.Join(root, "config"), "XDG_DATA_HOME": filepath.Join(root, "data"),
				"XDG_STATE_HOME": filepath.Join(root, "state"), "XDG_CACHE_HOME": filepath.Join(root, "cache"),
				"NVIM_APPNAME": "nvim", "NVIM_LOG_FILE": filepath.Join(root, "nvim.log"), "MAK_NVIM_CONFIG": config, "MAK_NVIM_PHASE": phase,
			} {
				cmd.Env = setEnv(cmd.Env, key, value)
			}
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			if _, err := os.Stat(filepath.Join(config, ".mak-"+phase+"-ok")); err != nil {
				t.Fatal("setup did not complete:", err)
			}
		})
	}
}
