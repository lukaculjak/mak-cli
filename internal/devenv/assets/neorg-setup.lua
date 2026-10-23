local errors = {}
local notify = vim.notify
vim.notify = function(message, level, opts)
  if level == vim.log.levels.ERROR then errors[#errors + 1] = tostring(message) end
  notify(message, level, opts)
end
local ok, failure = xpcall(function()
  local config = assert(vim.env.MAK_NVIM_CONFIG)
  vim.cmd.cd(config)
  vim.opt.rtp:prepend(config)
  vim.go.loadplugins = true
  dofile(config .. "/init.lua")
  require("lazy").install({ wait = true, show = false })
  require("neorg").org_file_entered(true)
  for _, language in ipairs({ "norg", "norg_meta" }) do
    assert(pcall(vim.treesitter.language.add, language), "Missing Neorg parser: " .. language)
  end
  assert(require("neorg").modules.get_module("core.journal"), "Neorg journal did not load")
  assert(vim.v.errmsg == "", vim.v.errmsg)
  assert(#errors == 0, table.concat(errors, "\n"))
end, debug.traceback)
if not ok then
  io.stderr:write("mak Neorg setup failed: " .. tostring(failure) .. "\n")
  vim.cmd("cquit 1")
end
vim.cmd("qa!")
