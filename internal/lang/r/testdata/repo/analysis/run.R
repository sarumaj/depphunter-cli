#!/usr/bin/env Rscript
suppressPackageStartupMessages(library(dplyr))
if (!require("readr")) stop("readr")
requireNamespace("jsonlite", quietly = TRUE)
pacman::p_load(tidyr, "DESeq2", mylocal)
pkg <- "ggplot2"
library(pkg, character.only = TRUE)
library(stats)
library(MASS)
library(survival)
loadNamespace("S4Vectors")
source("R/load.R")
source(here::here("R", "clean.R"))
sys.source(file.path("R", "missing.R"), envir = new.env())
source("https://example.org/remote.R")
box::use(
  stringr[str_detect, str_trim],
  ./modules/helpers,
  h = modules/helpers,
)
data <- load_data("x.csv")
data <- clean(data)
report <- function(d) summarise_all(d, mean)
