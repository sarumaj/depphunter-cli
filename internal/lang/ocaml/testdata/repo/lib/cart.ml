open Import

type item = { name : string; price : Money.t }
and t = item list

exception Empty of string

let empty = []

let add t item =
  let open Lwt.Syntax in
  ignore (let* () = Lwt.return_unit in Lwt.return_unit);
  item :: t

let rec total = function
  | [] -> Price.zero
  | { price; _ } :: rest -> Price.add price (total rest)

let to_json t : Json.t =
  `List (List.map (fun i -> `String i.name) t)

(* Constructors are not modules: Some, Ok, Empty, `Poly. *)
let first = function
  | x :: _ -> Some x
  | [] -> raise (Empty "no items")

let check = function Ok v -> Ok v | Error e -> Error (Strings.trim e)

let log t = Logs.info (fun m -> m "%a" Fmt.(list string) (List.map (fun i -> i.name) t))

module Id = struct
  type t = int
  let counter = ref 0
  let next () = incr counter; !counter
end

module Local = struct
  let x = 1
end

let uses_local = Local.x + Id.next ()

let unix_time () = Lwt_unix.sleep 1.0

let s = "Not.A_module in a string" and c = '"' (* Nor.This in a comment "*)" *)

let q = {|Quoted.Module|} ^ {id|Also.Not |id}

class counter = object
  val mutable n = 0
  method incr = n <- n + 1
  method get = n
end

external raw_hash : string -> int = "shop_hash"
