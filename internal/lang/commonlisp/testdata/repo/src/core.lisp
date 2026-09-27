(defpackage :shop.core
  (:use :cl :undeclared-pkg)
  (:import-from :local-time #:now)
  (:import-from :5am #:is)
  (:export #:now))
(in-package :shop.core)
