module Cart

using ..Shop: Item
using ..Pricing

total(items) = sum(price, items)

end
