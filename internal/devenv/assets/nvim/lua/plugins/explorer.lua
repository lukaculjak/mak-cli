return {
  {
    "folke/snacks.nvim",
    opts = {
      picker = {
        sources = {
          explorer = {
            -- Show environment files even when hidden or Git-ignored.
            include = { ".env", ".env.*" },
          },
        },
      },
    },
  },
}
