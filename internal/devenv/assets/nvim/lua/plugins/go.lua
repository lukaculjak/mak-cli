return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        -- Mason installs gopls without requiring an existing ~/go/bin binary.
        gopls = {},
      },
    },
  },
  {
    "nvim-treesitter/nvim-treesitter",
    opts = {
      ensure_installed = { "go", "gomod", "gowork", "gosum" },
    },
  },
}
