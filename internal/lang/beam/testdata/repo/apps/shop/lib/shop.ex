defmodule Shop do
  @moduledoc """
  Nothing in here is a reference: Phoenix.Router, :ets.new(), Fake.Module.
  """

  defstruct [:name]

  @type t :: %__MODULE__{name: String.t()}

  def start(name), do: GenServer.start_link(__MODULE__, name)
  def start(name, opts) when is_list(opts), do: {name, opts}
  def start(name, opts), do: {name, opts}

  defp secret?(x), do: ?" == x

  defmacro __using__(_opts) do
    quote do
      import Shop
    end
  end

  defmodule Error do
    defexception [:message]
  end

  def fail!, do: raise(Error, "boom")
end
