-- Run under -u NONE so errors are caught before loading the bundled configuration.
local errors = {}
local notify = vim.notify
vim.notify = function(message, level, opts)
  if level == vim.log.levels.ERROR then
    errors[#errors + 1] = tostring(message)
  end
  notify(message, level, opts)
end

local function check_errors()
  assert(vim.v.errmsg == "", vim.v.errmsg)
  assert(#errors == 0, table.concat(errors, "\n"))
end

local function plugin_opts(name)
  local plugin = assert(require("lazy.core.config").plugins[name], "Missing plugin: " .. name)
  return require("lazy.core.plugin").values(plugin, "opts", false)
end

local function log_tail(name)
  local path = vim.fn.stdpath("state") .. "/" .. name
  if vim.fn.filereadable(path) == 0 then return "" end
  local lines = vim.fn.readfile(path)
  return "\n" .. table.concat(vim.list_slice(lines, math.max(1, #lines - 12)), "\n")
end

local function install_tools()
  require("lazy").load({ plugins = { "mason.nvim", "nvim-treesitter" } })
  local registry = require("mason-registry")
  local refreshed, refresh_ok = false, false
  registry.refresh(function(ok)
    refresh_ok, refreshed = ok, true
  end)
  assert(vim.wait(300000, function() return refreshed end, 100), "Mason registry refresh timed out")
  assert(refresh_ok, "Mason registry refresh failed; check your network/GitHub access")

  -- Derive the complete tool list from the active LazyVim configuration.
  local packages = {}
  for _, name in ipairs(plugin_opts("mason.nvim").ensure_installed or {}) do
    packages[name] = true
  end
  local mapping = require("mason-lspconfig.mappings").get_mason_map().lspconfig_to_package
  for name, opts in pairs(plugin_opts("nvim-lspconfig").servers) do
    if name ~= "*" and opts ~= false and opts.enabled ~= false and opts.mason ~= false then
      packages[assert(mapping[name], "No Mason package for LSP: " .. name)] = true
    end
  end
  for _, name in ipairs(vim.tbl_keys(packages)) do
    local package = registry.get_package(name)
    if not package:is_installed() and not package:is_installing() then
      print("Installing Mason tool: " .. name)
      package:install()
    end
  end
  assert(vim.wait(900000, function()
    for name in pairs(packages) do
      if registry.get_package(name):is_installing() then return false end
    end
    return true
  end, 100), "Mason language server installation timed out")
  for name in pairs(packages) do
    assert(registry.get_package(name):is_installed(), "Mason failed to install " .. name .. log_tail("mason.log"))
  end

  local parsers = plugin_opts("nvim-treesitter").ensure_installed
  assert(require("nvim-treesitter").install(parsers, { max_jobs = 4, summary = true }):wait(900000), "Syntax parser installation failed")
  for _, language in ipairs(parsers) do
    assert(pcall(vim.treesitter.language.add, language), "Cannot load syntax parser: " .. language)
  end
end

local function verify_lsp()
  require("lazy").load({ plugins = { "nvim-lspconfig", "blink.cmp", "nvim-treesitter" } })
  assert(vim.g.colors_name == "gruvbox", "Gruvbox theme did not load")
  assert(require("blink.cmp").get_lsp_capabilities().textDocument.completion, "Completion capabilities missing")
  for _, tool in ipairs({ "node", "npm", "go", "ruby", "python3", "elixir", "ghc", "cabal", "haskell-language-server-wrapper", "rg", "fd", "tree-sitter" }) do
    assert(vim.fn.executable(tool) == 1, "Missing executable: " .. tool)
  end
  -- HLS is tied to the project's GHC version; verify the installed baseline matches.
  local hls = vim.system({ "haskell-language-server-wrapper", "--probe-tools" }, { text = true }):wait(60000)
  assert(hls.code == 0, "Haskell toolchain check failed: " .. (hls.stderr or ""))

  -- Initialize every enabled server: installation alone does not prove it can run.
  local servers = plugin_opts("nvim-lspconfig").servers
  -- Start clients explicitly; Vue also needs its TypeScript companion.
  for name, opts in pairs(servers) do
    if name ~= "*" and opts ~= false and opts.enabled ~= false then
      vim.lsp.enable(name, false)
    end
  end
  local samples = {
    cssls = { "css", "sample.css", "body { color: red; }" },
    html = { "html", "sample.html", "<!doctype html><html></html>" },
    jsonls = { "json", "sample.json", "{}" },
    vtsls = { "typescript", "sample.ts", "const message = 'hello';\nmessage;" },
    gopls = { "go", "main.go", 'package main\n\nconst message = "hello"\n\nfunc main() {\n\t_ = message\n}' },
    vue_ls = { "vue", "sample.vue", "<template><div>Hello</div></template>" },
    pyright = { "python", "sample.py", "message = 'hello'" },
    ruff = { "python", "sample.py", "message = 'hello'" },
    ruby_lsp = { "ruby", "sample.rb", "message = 'hello'" },
    rubocop = { "ruby", "sample.rb", "message = 'hello'" },
    hls = { "haskell", "Sample.hs", "module Sample where\nmessage = \"hello\"" },
    elixirls = { "elixir", "sample.ex", "defmodule Sample do\nend" },
    lua_ls = { "lua", "sample.lua", "local message = 'hello'" },
  }
  local root = vim.fn.stdpath("cache") .. "/mak-lsp-check"
  vim.fn.mkdir(root, "p")
  vim.fn.writefile({ '{"compilerOptions":{"strict":true},"include":["*.ts","*.vue"]}' }, root .. "/tsconfig.json")
  vim.fn.writefile({ '{"name":"mak-lsp-check","version":"1.0.0"}' }, root .. "/package.json")
  vim.fn.writefile({ "module mak-lsp-check", "", "go 1.23.0" }, root .. "/go.mod")
  vim.fn.writefile({ "defmodule MakCheck.MixProject do", "  use Mix.Project", '  def project, do: [app: :mak_check, version: "0.1.0", elixir: ">= 1.14.0"]', "end" }, root .. "/mix.exs")
  vim.fn.writefile({ "name: mak-check", "version: 0.1.0.0", "build-type: Simple", "cabal-version: >=1.10", "library", "  exposed-modules: Sample", "  build-depends: base", "  default-language: Haskell2010" }, root .. "/mak-check.cabal")
  for _, name in ipairs(vim.tbl_keys(servers)) do
    local opts = servers[name]
    if name ~= "*" and opts ~= false and opts.enabled ~= false then
      local sample = assert(samples[name], "Add a setup verification sample for LSP: " .. name)
      local path = root .. "/" .. sample[2]
      vim.fn.writefile(vim.split(sample[3], "\n"), path)
      local buf = vim.fn.bufadd(path)
      vim.fn.bufload(buf)
      vim.bo[buf].filetype = sample[1]
      local companion
      if name == "vue_ls" then
        local settings = vim.deepcopy(assert(vim.lsp.config.vtsls, "Vue requires vtsls"))
        settings.root_dir, settings.workspace_folders = root, nil
        local id = assert(vim.lsp.start(settings, { bufnr = buf }), "Vue TypeScript companion could not start")
        companion = assert(vim.lsp.get_client_by_id(id))
        assert(vim.wait(180000, function() return companion.initialized or companion:is_stopped() end, 100)
          and companion.initialized and not companion:is_stopped(), "Vue TypeScript companion failed to initialize")
      end
      local config = vim.deepcopy(assert(vim.lsp.config[name], "LSP is not configured: " .. name))
      config.root_dir, config.workspace_folders = root, nil
      config.capabilities = require("blink.cmp").get_lsp_capabilities(config.capabilities)
      config.capabilities.workspace.didChangeWatchedFiles = { dynamicRegistration = false }
      local id = assert(vim.lsp.start(config, { bufnr = buf }), "LSP could not start: " .. name)
      local client = assert(vim.lsp.get_client_by_id(id))
      assert(vim.wait(180000, function() return client.initialized or client:is_stopped() end, 100), "LSP initialization timed out: " .. name)
      assert(client.initialized and not client:is_stopped(), "LSP failed to initialize: " .. name .. log_tail("lsp.log"))
      if name == "cssls" or name == "vtsls" or name == "gopls" then
        assert(client.server_capabilities.completionProvider, name .. " has no completion support")
      end
      if name == "vtsls" or name == "gopls" then
        local label = name == "gopls" and "Go" or "TypeScript"
        local position = name == "gopls" and { line = 5, character = 7 } or { line = 1, character = 2 }
        local params = { textDocument = { uri = vim.uri_from_fname(path) }, position = position }
        local result, err = client:request_sync("textDocument/definition", params, 30000, buf)
        assert(result and not result.err and result.result and #result.result > 0, label .. " go-to-definition failed: " .. vim.inspect(err or result))
        params.position.character = position.character + 1
        result, err = client:request_sync("textDocument/completion", params, 30000, buf)
        local items = result and result.result and (result.result.items or result.result)
        assert(result and not result.err and items and #items > 0, label .. " completion failed: " .. vim.inspect(err or result))
      end
      print("Verified language server: " .. name)
      client:stop()
      assert(vim.wait(10000, function() return client:is_stopped() end, 100), "LSP did not stop: " .. name)
      if companion then companion:stop() end
    end
  end
  vim.fn.delete(root, "rf")
end

local ok, err = xpcall(function()
  assert(vim.fn.has("nvim-0.11.2") == 1, "This configuration requires Neovim 0.11.2 or newer; upgrade Homebrew's neovim and retry")
  local config = assert(vim.env.MAK_NVIM_CONFIG)
  local locked = vim.json.decode(table.concat(vim.fn.readfile(config .. "/lazy-lock.json"), "\n"))
  vim.cmd.cd(config)
  vim.opt.rtp:prepend(config)
  vim.go.loadplugins = true
  dofile(config .. "/init.lua")
  local phase = assert(vim.env.MAK_NVIM_PHASE)
  if phase == "plugins" then
    require("lazy").install({ wait = true, lockfile = true, show = false })
    require("lazy").restore({ wait = true, show = false })
    for name, plugin in pairs(require("lazy.core.config").plugins) do
      for _, task in ipairs(plugin._.tasks or {}) do
        assert(not task:has_errors(), "Plugin installation/build failed: " .. name)
      end
      local commit = assert(locked[name], "Plugin absent from lockfile: " .. name).commit
      local head = vim.system({ "git", "-C", plugin.dir, "rev-parse", "HEAD" }, { text = true }):wait()
      assert(head.code == 0 and vim.trim(head.stdout) == commit, "Plugin revision mismatch: " .. name)
    end
  elseif phase == "tools" then
    install_tools()
  elseif phase == "verify" then
    verify_lsp()
  else
    error("Unknown setup phase: " .. phase)
  end
  check_errors()
  vim.fn.writefile({ "ok" }, config .. "/.mak-" .. phase .. "-ok")
end, debug.traceback)
if not ok then
  io.stderr:write("mak Neovim setup failed: " .. tostring(err) .. "\n")
  vim.cmd("cquit 1")
end
vim.cmd("qa!")
