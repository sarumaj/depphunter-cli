(ns shop.common
  (:require #?(:clj  [clojure.edn :as edn]
               :cljs [cljs.reader :as edn])
            #?@(:clj  [[clojure.java.shell :as sh]]
                :cljs [[goog.object :as gobj]])
            [clojure.spec.alpha :as s])
  #?(:cljs (:require-macros [shop.common])))

#?(:clj  (defn now [] (java.util.Date.))
   :cljs (defn now [] (js/Date.)))

(defmacro when-shop [& body] `(do ~@body))
