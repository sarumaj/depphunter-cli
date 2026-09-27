(uiop:define-package :acme/util
  (:use :cl)
  (:mix :uiop/utility)
  (:export #:helper))
(in-package :acme/util)
(defun helper () t)
