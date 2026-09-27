defmodule Shop.Pricing.Rules do
  def apply(rules, price, _), do: {rules, price}
end

defmodule Shop.Cart.Line do
  defstruct items: []
  def new, do: %__MODULE__{}
end
