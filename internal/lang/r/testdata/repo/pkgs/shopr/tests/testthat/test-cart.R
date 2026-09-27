library(testthat)
library(shopr)

test_that("total adds up", {
  cart <- new_cart()
  expect_equal(total(cart), 0)
  expect_equal(shopr:::round_price(1.234), 1.23)
})
