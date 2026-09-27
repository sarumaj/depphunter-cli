(defpackage :shop/tests
  (:use :cl :rove :shop))
(in-package :shop/tests)

(deftest checkout-works
  (ok (shop:checkout nil)))
