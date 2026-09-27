(ns legacy.handler
  (:use compojure.core)
  (:require [clj-http.client :as http]
            [hiccup.page :refer [html5]]
            [ring.mock.request :as mock]
            [legacy.views :as views]))

(defroutes app (GET "/" [] (html5 [:p "hi"])))
