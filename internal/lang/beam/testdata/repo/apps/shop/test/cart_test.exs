defmodule Shop.CartTest do
  use ExUnit.Case, async: true
  use ExUnitProperties

  test "new" do
    assert Shop.Cart.new([])
  end
end
