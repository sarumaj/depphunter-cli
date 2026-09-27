defmodule ShopWeb.MixProject do
  use Mix.Project

  def project do
    [app: :shop_web, version: "0.1.0", deps: deps()]
  end

  defp deps do
    [
      {:phoenix, github: "phoenixframework/phoenix", branch: "main", override: true},
      {:phoenix_live_view, "~> 0.20.0"},
      {:shop, in_umbrella: true}
    ]
  end
end
