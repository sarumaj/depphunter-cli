(ns acme.shared.util
  (:require [medley.core :as m]
            [cheshire.core :as json]))

(defn ids [xs] (m/index-by :id xs))
