open Cmdliner

let cart = Cart.add Shop.Cart.empty { Shop.Cart.name = "tea"; price = Shop.Price.zero }
let words = Re.split (Re.compile (Re.rep1 Re.space)) "a b"
let re = Str.regexp "x"
let term = Term.(const ignore $ const ())
let cmd = Cmd.v (Cmd.info "shop") term
let () = exit (Cmd.eval cmd)
