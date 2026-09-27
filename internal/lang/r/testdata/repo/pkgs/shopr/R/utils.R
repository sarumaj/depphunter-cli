round_price <- function(x) round(x, 2)

price_of <- function(item) {
  if (is.null(item$price)) {
    helper_price <- function() 0 # not top level
    return(helper_price())
  }
  item$price
}

setClass("Receipt", representation(total = "numeric"))
setGeneric("describe", function(x) standardGeneric("describe"))
setMethod("describe", "Receipt", function(x) cat(x@total))
methods::setMethod("show", signature("Receipt"), function(object) describe(object))

Account <- setRefClass("Account",
  fields = list(balance = "numeric"),
  methods = list(
    deposit = function(x) balance <<- balance + x
  )
)
Account$methods(withdraw = function(x) balance <<- balance - x)

assign("make_receipt", function(t) new("Receipt", total = t))

stats_summary <- function(x) {
  stats::median(x)
  localhelper::helper()
  MASS::fitdistr(x, "normal")
  survival::Surv(x)
}
