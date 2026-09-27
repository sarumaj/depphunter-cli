(ns shop.ui
  (:require ["react" :as react]
            ["@mui/material/Button" :default Button]
            ["fs" :as fs]
            ["./local.js" :as local]
            ["lodash" :as lodash]
            [reagent.core :as r]
            [re-frame.core :as rf]
            [goog.string :as gstr]
            [cljs.core.async :refer [go]]
            [shop.common :as common]
            [left-pad :as lp])
  (:require-macros [shop.macros :refer [defview]])
  (:import [goog.net XhrIo]))

(defn app [] [:div "shop"])
