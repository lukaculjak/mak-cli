return {
  {
    "norcalli/nvim-colorizer.lua",
    event = { "BufReadPre", "BufNewFile" },
    config = function()
      require("colorizer").setup({ "*" }, {
        css = true,
        names = false,
        RRGGBBAA = true,
      })
    end,
  },
}
