(def hiccup-version "1.0.5")

(defproject acme/legacy "0.1.0-SNAPSHOT"
  :description "The old shop, on Leiningen."
  :dependencies [[org.clojure/clojure "1.10.3"]
                 [compojure "1.7.0"]
                 [clj-http "3.12.3" :exclusions [commons-logging]]
                 [hiccup ~hiccup-version]]
  :managed-dependencies [[hiccup "1.0.5"]]
  :plugins [[lein-ring "0.12.6"]]
  :profiles {:dev {:dependencies [[ring/ring-mock "0.4.0"]]}}
  :repositories [["internal" {:url "https://repo.acme.example/maven"}]]
  :source-paths ["src/clj"])
