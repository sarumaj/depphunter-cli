#lang scribble/manual
@(require (for-label racket/base shop)
          scribble/example)

@title{The Shop}

Install it and write (require fake/prose) in your program.

@racketblock[(require fake/example)]

@;{ (require fake/at-comment) @(require fake/at-comment-form) }
@; (require fake/at-line-comment)

@(define shop-eval 1)

@include-section["intro.scrbl"]
