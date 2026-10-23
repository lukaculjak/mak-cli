return {
  {
    "lewis6991/gitsigns.nvim",
    keys = {
      {
        "<leader>gv",
        function()
          require("gitsigns").toggle_current_line_blame()
        end,
        desc = "Toggle Git Line Blame",
      },
    },
    opts = {
      current_line_blame = true,
      current_line_blame_opts = { delay = 250 },
    },
  },
}
