module Shop.Domain.Pricing

let total (items: Item list) = items |> List.sumBy (fun i -> decimal i.Quantity)
