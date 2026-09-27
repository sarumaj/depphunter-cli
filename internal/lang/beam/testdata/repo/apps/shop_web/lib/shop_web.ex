defmodule ShopWeb do
  def controller do
    quote do
      use Phoenix.Controller, formats: [:html]
      alias ShopWeb.Router.Helpers, as: Routes
    end
  end

  defmacro __using__(which) when is_atom(which) do
    apply(__MODULE__, which, [])
  end
end
