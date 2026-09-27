defmodule ShopWeb.CartLive do
  use Phoenix.LiveView

  def render(assigns) do
    ~H"""
    <div>{Shop.Cart.total(@cart, [])}</div>
    """
  end

  def mount(_params, _session, socket), do: {:ok, :legacy_parser.parse(socket)}
end
