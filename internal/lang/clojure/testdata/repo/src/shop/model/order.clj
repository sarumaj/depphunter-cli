(ns shop.model.order
  (:require [shop.model.cart :refer [->Cart]]))

(defn order [] (->Cart []))
