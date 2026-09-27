(in-package #:shop)

(eval-when (:compile-toplevel :load-toplevel :execute)
  (defun helper (x) x))

(let ((counter 0))
  (defun next-id () (incf counter)))

#+sbcl (defun platform () :sbcl)
#-sbcl (defun platform () :other)

(defun checkout (cart)
  (bt:make-thread (lambda () (total cart)))
  (dex:get "https://example.com")
  (str:concat "a" "b")
  (shop.core:now)
  (local-time:now)
  (uiop:getenv "HOME")
  (sb-ext:gc)
  (mystery:thing)
  (ppcre:scan "x" "y"))
