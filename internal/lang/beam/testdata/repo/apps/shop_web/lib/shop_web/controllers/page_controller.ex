defmodule ShopWeb.PageController do
  use ShopWeb, :controller

  def index(conn, _params) do
    conn
    |> Plug.Conn.put_status(200)
    |> redirect(to: Routes.page_path(conn, :index))
  end
end
