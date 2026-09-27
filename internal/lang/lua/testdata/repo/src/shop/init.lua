-- The shop module.
local cart = require("shop.cart")
local utils = require 'pl.utils'
local http = require "socket.http"
local ok, cjson = pcall(require, "cjson")
local lpeg = require("lpeg")
local strings = require("string")
local opt = require("jit.opt")
local lfs = require("lfs")
local util = require("shop.util")
local ssl = require("ssl")
local req = require("resty.http")
local ngx_ssl = require("ngx.ssl")
local missing = require("unknownmod.x")
local native = require [[shop.native]]
dofile("scripts/setup.lua")
local nothing = loadfile("scripts/absent.lua")
local dynamic = require(cart.name)
local extra = require("extra.mod")

local M = {}

M.version = "1.0"

function M.add(item)
  local function nested() end
  return cart.add(item)
end

function M:checkout()
  if self.empty then
    return nil
  end
end

M.handler = function(self, event)
  return event
end

local function helper()
  local inner = {}
  return inner
end

if helper() then
  function M.maybe() end
end

M.version = "2.0"

return M
