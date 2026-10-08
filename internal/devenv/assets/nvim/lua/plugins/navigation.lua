return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        ["*"] = {
          keys = {
            {
              "gd",
              function()
                Snacks.picker.lsp_definitions({ confirm = "tab" })
              end,
              desc = "Goto Definition (New Tab)",
              has = "definition",
            },
          },
        },
      },
    },
  },
}
