return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        tailwindcss = {
          settings = {
            tailwindCSS = {
              validate = false,
            },
          },
        },
      },
    },
  },
}
