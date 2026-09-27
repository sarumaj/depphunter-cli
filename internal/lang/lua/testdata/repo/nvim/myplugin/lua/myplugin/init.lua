local config = require("myplugin.config")
local lsp = require("vim.lsp")
local async = require("plenary.async")
local love = require("love.graphics")

local M = {}

function M.setup(opts)
  config.apply(opts)
end

return M
