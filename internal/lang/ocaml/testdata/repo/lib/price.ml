type t = { cents : int } [@@deriving show]

let zero = { cents = 0 }
let add a b = { cents = a.cents + b.cents }
let ( +$ ) = add
let to_string t = Printf.sprintf "%d.%02d" (t.cents / 100) (t.cents mod 100)
