return {
  {
    "neovim/nvim-lspconfig",
    opts = function(_, opts)
      -- Use Apple's clangd so it shares the compiler's macOS SDK/toolchain.
      opts.servers.clangd.mason = false
      opts.servers.clangd.cmd[1] = "/usr/bin/clangd"
    end,
  },
  {
    "mason-org/mason.nvim",
    opts = { ensure_installed = { "clang-format" } },
  },
  {
    "stevearc/conform.nvim",
    opts = {
      formatters_by_ft = {
        c = { "clang_format" },
        cpp = { "clang_format" },
      },
    },
  },
}
