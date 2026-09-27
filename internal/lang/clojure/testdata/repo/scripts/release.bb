#!/usr/bin/env bb
(require '[babashka.fs :as fs]
         '[babashka.http-client :as http]
         '[cheshire.core :as json])

(defn -main [& args] (fs/exists? "x"))
