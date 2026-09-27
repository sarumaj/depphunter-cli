#!/usr/bin/env ocaml
#use "topfind";;
#require "yojson,lwt.unix";;
#if OCAML_VERSION >= (4, 14, 0)
let json = Yojson.Safe.from_string "{}"
#endif
let () = print_endline (Yojson.Safe.to_string json)
