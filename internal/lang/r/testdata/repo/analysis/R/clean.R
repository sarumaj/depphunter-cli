clean <- function(d) {
  d |> tidyr::drop_na()
}
