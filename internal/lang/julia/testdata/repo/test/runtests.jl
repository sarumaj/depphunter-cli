using Test, Shop, Aqua
using Utils.Strings
import Shop.Cart
using Missing1

includet("helpers.jl")
include("$(@__DIR__)/helpers.jl")
include(joinpath(dirname(@__FILE__), "..", "scripts", "report.jl"))
for f in readdir()
    include(f)
end

@testset "shop" begin
    @test Shop.checkout(Shop.Basket([])) == 0
end
