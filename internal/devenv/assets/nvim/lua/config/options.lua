-- Options are automatically loaded before lazy.nvim startup
-- Default options that are always set: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/options.lua
-- Add any additional options here
vim.g.autoformat = false

-- Keep version-manager and project runtimes ahead of Homebrew fallbacks.
local prefix = vim.fn.readfile(vim.fn.stdpath("config") .. "/mak-brew-prefix")[1]
local fallback = {
  prefix .. "/opt/ruby/bin",
  prefix .. "/opt/python/bin",
  prefix .. "/opt/python/libexec/bin",
}
local versioned = vim.fn.glob(prefix .. "/opt/python@*/bin", false, true)
local function minor(path) return tonumber(path:match("python@3%.(%d+)/bin$")) or -1 end
table.sort(versioned, function(a, b) return minor(a) > minor(b) end)
for _, path in ipairs(versioned) do
  if minor(path) >= 0 and vim.fn.executable(path .. "/python3") == 1 then
    fallback[#fallback + 1] = path
  end
end
vim.list_extend(fallback, { prefix .. "/bin", prefix .. "/sbin" })
local paths, inserted = {}, false
for _, path in ipairs(vim.split(vim.env.PATH or "", ":", { plain = true })) do
  if not vim.tbl_contains(fallback, path) then
    if not inserted and vim.tbl_contains({ "/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/local/bin", "/usr/local/sbin" }, path) then
      vim.list_extend(paths, fallback)
      inserted = true
    end
    paths[#paths + 1] = path
  end
end
if not inserted then vim.list_extend(paths, fallback) end
vim.env.PATH = table.concat(paths, ":")
