(ns shop.core
  "Shop entry point."
  {:author "shop"}
  (:require [clojure.string :as str]
            [clojure.java.io :as io]
            [cheshire.core :as json]
            [ring.util.response :as resp]
            [next.jdbc :as jdbc]
            [reitit.ring :as rr]
            [taoensso.timbre :as log]
            [clojure.core.async :as a]
            [clojure.spec.alpha :as s]
            [clojure.data.json :as dj]
            [acme.widgets.api :as w]
            [acme.shared.util :as u]
            [shop.db-util :as db]
            [shop.model [cart :as cart] [order]]
            [shop.alias-only :as-alias ao]
            #_[shop.never :as never]
            [unknown.lib :as ul]
            [shop.gone :as gone])
  (:import [java.util Date UUID]
           (java.io File)
           org.eclipse.jetty.server.Server
           [shop.model.cart Cart]
           shop.Native
           clojure.lang.IFn
           com.example.Nothing))

(defn- helper [x] x)

(defn start
  "Starts the shop."
  ([] (start 8080))
  ([port] (helper port)))

(def ^:dynamic *config* {})

(defonce state (atom nil))

(defmacro with-shop [& body] `(do ~@body))

(defmulti area :shape)
(defmethod area :circle [c] (:r c))
(defmethod area [:square :big] [c] (:side c))

(defprotocol Priced
  (price [x])
  (discount [x pct]))

(defrecord Item [name])
(deftype Box [v])

(comment
  (defn scratch [] :not-a-symbol))

(s/def ::id int?)

(require '[clojure.set :as set])

(load "core_extra")

(let [c \) s "(not a form"] [c s #"[(]" #inst "2024-01-01T00:00:00Z"])

(def ^{:doc "after all the noise"} last-one 1)
