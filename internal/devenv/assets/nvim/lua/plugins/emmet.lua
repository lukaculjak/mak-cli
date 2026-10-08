return {
  {
    "mattn/emmet-vim",
    lazy = false,
    init = function()
      vim.g.user_emmet_install_global = 0
      vim.g.user_emmet_mode = "i"
    end,
    config = function()
      vim.api.nvim_create_autocmd("FileType", {
        group = vim.api.nvim_create_augroup("EmmetWebFiles", { clear = true }),
        pattern = { "html", "css", "scss", "vue", "eruby", "javascriptreact", "typescriptreact" },
        callback = function()
          vim.cmd.EmmetInstall()
        end,
      })
    end,
  },
}
