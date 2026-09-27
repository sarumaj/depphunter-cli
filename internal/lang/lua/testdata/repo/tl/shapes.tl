local record Point
  x: number
  y: number
  add: function(Point, Point): Point
  record Nested
    z: number
  end
end

local enum Color
  "red"
  "green"
end

local type Callback = function(number): string

global function make(x: number, y: number): Point
  return { x = x, y = y }
end

local function helper(): Color
  return "red"
end

return { make = make }
