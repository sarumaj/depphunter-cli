{
open Parser
exception Error of string
}

let letter = ['a'-'z' 'A'-'Z']

rule token = parse
  | letter+ as n { NAME n }
  | eof { EOF }
  | '\'' { token lexbuf }
  | _ { raise (Error (Lexing.lexeme lexbuf)) }

and comment = parse
  | "*)" { () }
