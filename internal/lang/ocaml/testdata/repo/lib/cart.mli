(** A cart of items. *)

type item = { name : string; price : Price.t }
type t

val empty : t
val add : t -> item -> t
val total : t -> Price.t
val to_json : t -> Yojson.Safe.t
exception Empty of string
module Id : sig
  type t = int
  val next : unit -> t
end
