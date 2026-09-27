(ns shop.core-test
  (:require [clojure.test :refer [deftest is]]
            [shop.core :as core]
            [kaocha.repl :as k]
            [ring.mock.request :as mock]))

(deftest starts
  (is (= 8080 (core/start))))
