;;;; shop.asd - the shop system, its core and its tests
(defpackage :shop-asd (:use :cl :asdf))
(in-package :shop-asd)

#| (defsystem "fake-in-comment" :depends-on ("fake")) |#

(defsystem "shop"
  :version "1.0.0"
  :defsystem-depends-on ("cffi-grovel")
  :depends-on ("alexandria"
               (:version "cl-ppcre" "2.1")
               (:feature :sbcl "sb-posix")
               (:feature (:not :sbcl) "osicat")
               (:require "sb-introspect")
               "shop/core"
               "uiop"
               "dexador"
               "str"
               "cl-json"
               "bordeaux-threads"
               #+nil "commented-out"
               #+(or) "also-commented-out"
               "tree-sitter-cl"
               "cl-ppcre-unicode")
  :pathname "src/"
  :components ((:file "package")
               (:module "model"
                :components ((:file "item" :depends-on ("package"))
                             (:file "cart")))
               (:module "io" :pathname "input-output"
                :components ((:file "reader")))
               (:static-file "README.md" :pathname "../README.md")
               (:cffi-grovel-file "grovel")
               (:file "legacy" :pathname "old/legacy")
               (:file "missing"))
  :in-order-to ((test-op (test-op "shop/tests"))))

(defsystem "shop/core"
  :depends-on ("local-time")
  :components ((:file "src/core")))

(defsystem "shop/tests"
  :depends-on ("shop" "rove")
  :components ((:module "t" :components ((:file "main")))))
