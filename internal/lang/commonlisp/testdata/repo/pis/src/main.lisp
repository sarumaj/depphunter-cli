(defpackage :acme/main
  (:use :cl)
  (:import-from :acme/util #:helper)
  (:import-from :acme/extra)
  (:import-from :alexandria #:when-let)
  (:import-from :c2mop #:class-slots)
  (:import-from :mystery-lib))
(in-package :acme/main)
