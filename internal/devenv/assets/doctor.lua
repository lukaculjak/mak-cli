-- Run with -u NONE inside mak's read-only macOS sandbox.
local results = {}
local function save(complete)
  vim.fn.writefile({ vim.json.encode({ checks = results, complete = complete or false }) }, vim.env.MAK_DOCTOR_RESULT)
end
local function check(label, fn, repair)
  local ok, detail = pcall(fn)
  results[#results + 1] = { status = ok and "ok" or "error", label = label, detail = tostring(detail or ""), repair = ok and "" or repair }
  save()
  return ok
end
local repair = "mak setup dev (backs up and replaces the current Neovim environment)"
local errors = {}
vim.notify = function(message, level)
  if level == vim.log.levels.ERROR then errors[#errors + 1] = tostring(message) end
end

local function opts(name)
  return require("lazy.core.plugin").values(assert(require("lazy.core.config").plugins[name], "Missing plugin: " .. name), "opts", false)
end

local ok, failure = xpcall(function()
  if not check("Neovim version", function()
    assert(vim.fn.has("nvim-0.11.2") == 1, "Neovim 0.11.2 or newer is required")
    return tostring(vim.version())
  end, "brew upgrade neovim") then return end

  local config, data = vim.fn.stdpath("config"), vim.fn.stdpath("data")
  local locked = vim.json.decode(table.concat(vim.fn.readfile(config .. "/lazy-lock.json"), "\n"))
  vim.cmd.cd(config)
  vim.opt.rtp:prepend(config)
  vim.opt.rtp:prepend(data .. "/lazy/lazy.nvim")
  local lazy = require("lazy")
  local setup = lazy.setup
  lazy.setup = function(settings)
    settings.install = vim.tbl_extend("force", settings.install or {}, { missing = false })
    settings.checker = { enabled = false }
    settings.change_detection = { enabled = false }
    settings.local_spec = false
    vim.list_extend(settings.spec, {
      { "mason-org/mason.nvim", config = function(_, options)
        require("mason").setup(options)
        -- Read the installed registry; never refresh or install from it.
        require("mason-registry").refresh = function(callback)
          if callback then callback(true, {}) end
        end
      end },
      { "nvim-treesitter/nvim-treesitter", config = function(_, options)
        require("nvim-treesitter").setup(options)
      end },
    })
    return setup(settings)
  end
  if not check("Neovim configuration startup", function()
    vim.go.loadplugins = true
    dofile(config .. "/init.lua")
    vim.wait(100)
    assert(vim.v.errmsg == "", vim.v.errmsg)
    assert(#errors == 0, table.concat(errors, "; "))
    assert(vim.g.colors_name == "gruvbox", "Gruvbox theme did not load")
    return "Configuration and Gruvbox theme load successfully"
  end, repair) then return end

  for _, name in ipairs(vim.fn.sort(vim.tbl_keys(require("lazy.core.config").plugins))) do
    local plugin = require("lazy.core.config").plugins[name]
    check("Plugin: " .. name, function()
      assert(vim.fn.isdirectory(plugin.dir) == 1, "Plugin directory is missing")
      local revision = assert(locked[name], "Plugin is absent from lazy-lock.json").commit
      local head = vim.system({ "git", "-C", plugin.dir, "rev-parse", "HEAD" }, { text = true }):wait(5000)
      assert(head.code == 0 and vim.trim(head.stdout) == revision, "Installed revision differs from lazy-lock.json")
      return revision:sub(1, 12)
    end, repair)
  end

  require("lazy").load({ plugins = { "mason.nvim" } })
  local registry = require("mason-registry")
  for _, name in ipairs(opts("mason.nvim").ensure_installed or {}) do
    check("Mason tool: " .. name, function()
      assert(registry.get_package(name):is_installed(), "Tool is not installed")
      return "Installed"
    end, repair)
  end
  require("lazy").load({ plugins = { "nvim-lspconfig", "blink.cmp", "nvim-treesitter" } })
  check("Autocompletion", function()
    assert(require("blink.cmp").get_lsp_capabilities().textDocument.completion, "Completion capabilities are missing")
    return "blink.cmp provides LSP completion capabilities"
  end, repair)
  for _, language in ipairs(opts("nvim-treesitter").ensure_installed or {}) do
    check("Syntax parser: " .. language, function()
      local loaded, err = pcall(vim.treesitter.language.add, language)
      assert(loaded, tostring(err))
      return "Loads successfully"
    end, repair)
  end

  local servers = opts("nvim-lspconfig").servers
  local samples = {
    cssls = { "css", "sample.css", "body { color: red; }" },
    html = { "html", "sample.html", "<!doctype html><html></html>" },
    jsonls = { "json", "sample.json", "{}" },
    vtsls = { "typescript", "sample.ts", "const message = 'hello';\nmessage;" },
    gopls = { "go", "main.go", 'package main\nfunc main() {}' },
    vue_ls = { "vue", "sample.vue", "<template><div>Hello</div></template>" },
    pyright = { "python", "sample.py", "message = 'hello'" },
    ruff = { "python", "sample.py", "message = 'hello'" },
    ruby_lsp = { "ruby", "sample.rb", "message = 'hello'" },
    rubocop = { "ruby", "sample.rb", "message = 'hello'" },
    hls = { "haskell", "Sample.hs", "module Sample where\nmessage = \"hello\"" },
    elixirls = { "elixir", "sample.ex", "defmodule Sample do\nend" },
    lua_ls = { "lua", "sample.lua", "local message = 'hello'" },
  }
  local root = vim.env.MAK_DOCTOR_WORKSPACE
  vim.fn.mkdir(root, "p")
  vim.fn.writefile({ '{"compilerOptions":{"strict":true},"include":["*.ts","*.vue"]}' }, root .. "/tsconfig.json")
  vim.fn.writefile({ '{"name":"mak-doctor","version":"1.0.0"}' }, root .. "/package.json")
  vim.fn.writefile({ "module mak-doctor", "", "go 1.23.0" }, root .. "/go.mod")
  vim.fn.writefile({ "defmodule MakDoctor.MixProject do", "  use Mix.Project", '  def project, do: [app: :mak_doctor, version: "0.1.0", elixir: ">= 1.14.0"]', "end" }, root .. "/mix.exs")
  vim.fn.writefile({ "name: mak-doctor", "version: 0.1.0.0", "build-type: Simple", "cabal-version: >=1.10", "library", "  exposed-modules: Sample", "  build-depends: base", "  default-language: Haskell2010" }, root .. "/mak-doctor.cabal")
  for name, options in pairs(servers) do
    if name ~= "*" and options ~= false and options.enabled ~= false then vim.lsp.enable(name, false) end
  end
  local mapping = require("mason-lspconfig.mappings").get_mason_map().lspconfig_to_package
  for _, name in ipairs(vim.fn.sort(vim.tbl_keys(servers))) do
    local options = servers[name]
    if name ~= "*" and options ~= false and options.enabled ~= false then
      local client, companion
      check("Language server: " .. name, function()
        if options.mason ~= false then
          assert(registry.get_package(assert(mapping[name], "No Mason mapping")):is_installed(), "Language server is not installed")
        end
        local sample = assert(samples[name], "No diagnostic sample for this custom language server")
        local path = root .. "/" .. sample[2]
        vim.fn.writefile(vim.split(sample[3], "\n"), path)
        local buf = vim.fn.bufadd(path)
        vim.fn.bufload(buf)
        vim.api.nvim_set_current_buf(buf)
        vim.bo[buf].filetype = sample[1]
        if name == "vue_ls" then
          local settings = vim.deepcopy(assert(vim.lsp.config.vtsls, "Vue requires vtsls"))
          settings.root_dir, settings.workspace_folders = root, nil
          local id = assert(vim.lsp.start(settings, { bufnr = buf }), "Vue TypeScript companion could not start")
          companion = assert(vim.lsp.get_client_by_id(id))
          assert(vim.wait(30000, function() return companion.initialized or companion:is_stopped() end, 100)
            and companion.initialized and not companion:is_stopped(), "Vue TypeScript companion failed to initialize")
        end
        local settings = vim.deepcopy(assert(vim.lsp.config[name], "Server is not configured"))
        if name == "lua_ls" and type(settings.cmd) == "table" then
          vim.list_extend(settings.cmd, { "--logpath=" .. root .. "/lua-logs", "--metapath=" .. root .. "/lua-meta" })
        end
        settings.root_dir, settings.workspace_folders = root, nil
        settings.capabilities = require("blink.cmp").get_lsp_capabilities(settings.capabilities)
        settings.capabilities.workspace.didChangeWatchedFiles = { dynamicRegistration = false }
        local id = assert(vim.lsp.start(settings, { bufnr = buf }), "Could not start the language server")
        client = assert(vim.lsp.get_client_by_id(id))
        assert(vim.wait(30000, function() return client.initialized or client:is_stopped() end, 100), "Initialization timed out")
        if not client.initialized or client:is_stopped() then
          local log = vim.lsp.log.get_filename()
          local lines = vim.fn.filereadable(log) == 1 and vim.fn.readfile(log) or {}
          error("Language server exited before initialization. " .. table.concat(vim.list_slice(lines, math.max(1, #lines - 5)), "\n"))
        end
        return "Starts and completes LSP initialization"
      end, repair)
      if client then
        client:stop()
        if not vim.wait(3000, function() return client:is_stopped() end, 100) then client:stop(true) end
      end
      if companion then companion:stop() end
    end
  end
  check("Neovim runtime", function()
    assert(#errors == 0, table.concat(errors, "; "))
    return "No runtime errors reported"
  end, repair)
end, debug.traceback)
if not ok then
  check("Neovim diagnostics", function() error(failure) end, repair)
end
save(true)
vim.cmd("qa!")
