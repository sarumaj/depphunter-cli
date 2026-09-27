defmodule Shop.MixProject do
  use Mix.Project

  def project do
    [
      app: :shop,
      version: "0.1.0",
      build_path: "../../_build",
      lockfile: "../../mix.lock",
      deps: deps()
    ]
  end

  defp deps do
    [
      {:ecto_sql, "~> 3.10"},
      {:jason, "== 1.4.1"},
      {:elixir_uuid, "~> 1.2"},
      {:nimble_csv, "1.2.0"},
      {:local_lib, path: "../../libs/local_lib"},
      {:money, github: "elixirmoney/money", tag: "v1.12.0"}
    ]
  end
end
