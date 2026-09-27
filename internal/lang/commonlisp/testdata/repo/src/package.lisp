;;; The shop package.
(in-package :cl-user)

(defpackage #:shop
  (:use #:cl #:alexandria)
  (:nicknames #:shop-app)
  (:import-from #:cl-ppcre #:scan)
  (:shadowing-import-from :json #:encode-json)
  (:local-nicknames (#:a #:alexandria) (:bt :bordeaux-threads) (:ts :tree-sitter))
  (:export #:checkout))
