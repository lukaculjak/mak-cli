-- Options are automatically loaded before lazy.nvim startup
-- Default options that are always set: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/options.lua
-- Add any additional options here
vim.g.autoformat = false

-- Keep Homebrew's keg-only Ruby/Python available to Mason and language servers.
local prefix = vim.fn.readfile(vim.fn.stdpath("config") .. "/mak-brew-prefix")[1]
vim.env.PATH = table.concat({
  prefix .. "/opt/ruby/bin",
  prefix .. "/opt/python/libexec/bin",
  prefix .. "/bin",
  prefix .. "/sbin",
  vim.env.PATH or "",
}, ":")
