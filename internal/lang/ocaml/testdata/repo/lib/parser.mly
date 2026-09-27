%{
open Cart
let mk name = { name; price = Price.zero }
%}

/* Tokens don't need modules. */
%token <string> NAME
%token EOF
%start <Cart.item list> items

%%

items:
  | l = list(item) EOF { l }

%public item:
  | n = NAME { mk n }
