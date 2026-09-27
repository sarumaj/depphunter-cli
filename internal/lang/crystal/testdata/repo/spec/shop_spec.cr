require "spec"
require "spectator"
require "../src/shop"
require "shop/cart"

describe Shop do
  it "works" do
    Shop::VERSION.should eq("0.1.0")
  end
end
