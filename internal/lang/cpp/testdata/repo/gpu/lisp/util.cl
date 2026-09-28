;;;; Common Lisp, not an OpenCL kernel.
(defpackage :util (:use :cl))
(in-package :util)
(defun hello () (format t "~a" "#include <CL/cl.h>"))
