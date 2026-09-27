(in-package :shop)

(defstruct (item (:conc-name item-)) name price)

(defclass cart ()
  ((items :initform nil :accessor cart-items)))

(defgeneric total (thing))

(defmethod total ((c cart))
  (reduce #'+ (cart-items c) :key #'item-price))

(defmethod total :around ((c cart) &optional verbose)
  (declare (ignore verbose))
  (call-next-method))

(defmethod print-object ((i item) stream)
  (format stream "#<item ~a>" (item-name i)))

(defmethod (setf total) (new (c cart)) new)

(defun (setf item-label) (new i) (setf (item-name i) new))

(define-condition out-of-stock (error) ())

(deftype price () '(integer 0))

(defvar *carts* (a:make-hash-table*))
(defparameter *tax* 19)
(defconstant +max+ 10)

(defmacro with-cart ((var) &body body) `(let ((,var (make-instance 'cart))) ,@body))
