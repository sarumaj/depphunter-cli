pub external type Pid

pub external fn system_time() -> Int =
  "erlang" "system_time"

external fn log(String) -> Nil = "./legacy_ffi.mjs" "log"
