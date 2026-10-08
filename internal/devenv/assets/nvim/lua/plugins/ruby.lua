return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        -- Mason supplies Ruby LSP; no laptop-specific rbenv shim is required.
        ruby_lsp = {},
      },
    },
  },
}
