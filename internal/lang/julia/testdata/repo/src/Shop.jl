"""
    Shop

A shop: `using Shop` then `checkout(cart)`.
"""
module Shop

using JSON, HTTP
using LinearAlgebra: norm, dot
import Statistics as Stats
using Base.Threads: @spawn
import Base: show, +

export Item, checkout

include("cart.jl")
include(joinpath(@__DIR__, "internal", "pricing.jl"))

using .Pricing
using .Cart: total

const VERSION_TAG = "0.3"
const A, B = 1, 2

abstract type AbstractItem end
primitive type Money 64 end

struct Item <: AbstractItem
    name::String
    price::Float64
    Item(name) = new(name, 0.0)
end

mutable struct Basket{T}
    items::Vector{T}
end

@enum Color red green blue

macro audit(ex)
    return :(println($(string(ex))); $(esc(ex)))
end

function checkout(b::Basket)
    x = [i for i in b.items if i.price > 0]
    y = b.items[end]
    inner(z) = z + 1
    total(x)
end

function checkout(b::Basket, n)
    checkout(b)
end

Base.show(io::IO, i::Item) = print(io, i.name)
Base.:+(a::Item, b::Item) = Item(a.name * b.name)
discount(x::Real)::Float64 = x * 0.9
scale(x::T) where {T<:Real} = 2x
@inline fast(x) = x
@test_not_a_def f(x) == 1

let cache = Dict()
    global lookup(k) = get(cache, k, nothing)
end

if Sys.iswindows()
    winonly() = 1
end

s = """
module NotAModule
using NotAPackage
"""
c = 'x'
t = x'
#= a #= nested =# comment with using Hidden =#
end # module
