defmodule Shop.Cart do
  use GenServer
  require Logger
  alias Ecto.Changeset
  alias Shop.{Item, Pricing.Rules}
  alias __MODULE__.Line, as: CartLine
  import Ecto.Query, only: [from: 2]

  @behaviour :gen_statem

  def new(items \\ []) do
    %CartLine{}
    |> Changeset.change(%{items: items})
    |> Jason.encode!()
  end

  def total(%__MODULE__{} = cart, rules) do
    :ets.lookup(:carts, cart)
    Rules.apply(rules, Item.price(cart), ~s(Decimal.new(1)))
    UUID.uuid4()
    NimbleCSV.RFC4180.parse_string("a,b")
    Ecto.Adapters.SQL.query!(Shop.Repo, "select 1")
    LocalLib.help()
    Money.new(1, :EUR)
    Unknown.Thing.call()
    :crypto.hash(:sha256, "#{Fake.Interpolated.x()}")
  end

  defprotocol Priced do
    def price(item)
  end

  defimpl Priced, for: Shop.Item do
    def price(item), do: item.price
  end
end
