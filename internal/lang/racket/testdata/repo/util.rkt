#lang typed/racket
(require/typed "sub/helpers.rkt" [greet (-> String String)])
(require/opaque-type Moment moment? gregor)
(provide total)
(define-type Price Nonnegative-Integer)
(: total (-> (Listof Price) Price))
(define (total xs) (apply + xs))

(module helpers racket/base
  (require racket/format)
  (define (fmt x) (~a x)))
