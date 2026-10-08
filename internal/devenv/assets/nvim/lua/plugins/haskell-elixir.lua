return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        hls = {
          mason = false,
          cmd = { "haskell-language-server-wrapper", "--lsp" },
        },
        elixirls = {},
      },
    },
  },
  {
    "nvim-treesitter/nvim-treesitter",
    opts = {
      ensure_installed = { "haskell", "elixir", "heex", "eex" },
    },
  },
}
