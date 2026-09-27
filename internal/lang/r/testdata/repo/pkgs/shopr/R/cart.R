#' @include utils.R
#' @importFrom glue glue
NULL

#' A cart
#' @import R6
Cart <- R6::R6Class("Cart",
  public = list(
    items = NULL,
    initialize = function(items = list()) {
      self$items <- items
    },
    add = function(item) {
      self$items <- c(self$items, item)
      invisible(self)
    }
  ),
  private = list(
    secret = \(x) x
  )
)

total <- function(cart) {
  prices <- vapply(cart$items, price_of, numeric(1))
  dplyr::summarise(data.frame(p = prices), s = sum(p))
  round_price(sum(prices))
}

`%+%` <- function(a, b) paste(a, b)

discount = function(x, rate = 0.1) {
  shopr:::round_price(x * (1 - rate))
}

tax_rate <- 0.2
tax_rate <- 0.25

new_cart <- function() Cart$new()
