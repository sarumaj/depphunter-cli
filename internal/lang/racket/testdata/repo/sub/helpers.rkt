#lang at-exp racket/base
(require racket/require
         (multi-in racket (format string))
         (path-up "util.rkt"))
(provide greet)
(define (greet name)
  @string-append{Hello @name, (require fake/at-text) @"!"})
@(define shout 1)
