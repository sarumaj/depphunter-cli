defmodule Shop.Schema do
  defmacro __using__(_) do
    quote do
      use Ecto.Schema
      alias Ecto.Multi
      alias Shop.{Cart, Item}
    end
  end
end
