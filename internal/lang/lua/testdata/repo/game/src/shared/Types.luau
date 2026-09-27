export type Item = { id: number, name: string }
type Pair<T> = { first: T, second: T }

local M = {}

local function describe(item: Item): string
  local label = if item.id > 0 then "item" else "none"
  return `{label} {item.name} end`
end

local count: number = if M then 1 else 0

function M.after(pair: Pair<Item>): Item
  return pair.first
end

return M
