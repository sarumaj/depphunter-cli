#lang info

(define collection "shop")
(define version "1.2")

(define deps '("base"
               ["rackunit-lib" #:version "1.10"]
               "threading"
               ("gregor-lib" #:version "0.3")
               "https://github.com/jackfirth/rebellion.git#0123456789abcdef0123456789abcdef01234567"
               "git://github.com/acme/guard#main"
               "https://git.acme.dev/racket/acme-log.git?path=acme-log-lib#v1.4.0"
               ("typed-racket" #:checksum "fedcba9876543210fedcba9876543210fedcba98")
               "widgets-lib"
               "../outside-pkg"
               ; "commented-out-lib"
               #;"datum-commented-lib"))

(define build-deps (list "scribble-lib" "racket-doc"))

(define scribblings '(("docs/manual.scrbl" ())))
