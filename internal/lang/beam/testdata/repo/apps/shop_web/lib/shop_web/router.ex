defmodule ShopWeb.Router do
  use Phoenix.Router
  import Phoenix.LiveView.Router

  scope "/", ShopWeb do
    get "/", PageController, :index
    live "/cart", CartLive

    scope "/admin", Admin, as: :admin do
      get "/", DashboardController, :show
    end
  end
end
