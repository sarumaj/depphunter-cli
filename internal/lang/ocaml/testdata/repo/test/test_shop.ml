let test_total () =
  Alcotest.(check string) "empty" "0.00" (Shop.Price.to_string (Shop.Cart.total Shop.Cart.empty))

let () = Alcotest.run "shop" [ ("cart", [ Alcotest.test_case "total" `Quick test_total ]) ]
let _ = Yojson.Safe.to_string `Null
let _ = QCheck.Gen.int
