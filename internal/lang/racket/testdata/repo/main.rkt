#lang racket/base
;; The shop's entry point. (require fake/line-comment)
#| a block comment (require fake/block)
   #| nested |# (require fake/nested) |#
#;(require fake/datum-comment)

(require racket/list
         "util.rkt"
         "sub/helpers.rkt"
         (only-in racket/string string-join)
         (prefix-in t: threading)
         (rename-in rackunit [check r:check])
         (except-in racket/match match*)
         (for-syntax racket/base syntax/parse)
         (for-label racket/contract)
         (submod "." inner)
         (submod "sub/extra.rkt" tool)
         (file "/opt/racket/shared.rkt")
         (lib "gregor/main.rkt")
         (planet jaymccarthy/sqlite:5:1)
         shop/util
         widgets/button
         rebellion/collection/list
         acme-log/core
         guard
         typed/racket/unsafe
         net/url
         net/smtp
         plot/pict
         mystery/thing
         data/queue
         data/gvector
         compat)

(define greeting #<<EOS
(require fake/here-string)
EOS
  )
(define pattern #rx"(require fake/regex)")
(define open-paren #\()
(define space #\space)
(define label "(require fake/string)")
(define (checkout cart #:tax [tax 0])
  (+ (length cart) tax))
(define ((adder n) m) (+ n m))
(define-values (low high) (values 1 2))
(define-syntax-rule (swap! a b) (let ([t a]) (set! a b) (set! b t)))
(struct item (name price) #:transparent)
(define-struct (sale item) (discount))
(define cart%
  (class object%
    (super-new)
    (define/public (total) 0)
    (define/private (log-line) 0)))
(define prices #hash((apple . 1) ("pear" . 2)))
(define sizes #(1 2 3))
(define |odd name| 'quoted)
(define 3d-view 'three)
(define show (λ (x) x))
'(require fake/quoted)

(module inner racket/base
  (provide x)
  (define x 1))

(module+ test
  (require rackunit)
  (define helper 1))

(module+ test
  (require racket/function))

(module* main #f
  (define entry 1))
