defmodule Shop.Item do
  use Shop.Schema

  schema "items" do
    belongs_to :cart, Cart
  end

  def price(item), do: Multi.new() && item
  def line, do: Shop.Cart.Line.new()
end
