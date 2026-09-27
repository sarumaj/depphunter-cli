package r

import "strings"

// basePkgs are the packages of priority "base": they ship with R itself, are
// versioned with it and are on no repository.
//
// Implements: REQ-R-006
var basePkgs = set("base compiler datasets grDevices graphics grid methods parallel splines stats stats4 tcltk tools utils")

// recommendedPkgs ship with every binary R installation too, but are ordinary CRAN
// packages updated on their own: they are the project's dependency when it declares
// or locks one, and part of R otherwise (REQ-R-006).
var recommendedPkgs = set("MASS lattice Matrix nlme survival boot cluster codetools foreign KernSmooth rpart class nnet spatial mgcv")

// biocPkgs are Bioconductor's infrastructure and most-used packages. Without a lock
// that records its source, a package is only known to come from Bioconductor rather
// than CRAN by name.
//
// Implements: REQ-R-007
var biocPkgs = set(`Biobase BiocGenerics S4Vectors IRanges GenomicRanges GenomeInfoDb
SummarizedExperiment SingleCellExperiment DelayedArray HDF5Array XVector Biostrings BSgenome
GenomicFeatures GenomicAlignments Rsamtools rtracklayer AnnotationDbi AnnotationHub ExperimentHub
BiocParallel BiocFileCache BiocIO BiocStyle BiocCheck MultiAssayExperiment DESeq2 edgeR limma
scran scater scuttle org.Hs.eg.db org.Mm.eg.db GO.db KEGGREST biomaRt clusterProfiler
ComplexHeatmap sva VariantAnnotation MatrixGenerics zlibbioc Rhtslib beachmat bluster
batchelor MAST topGO DOSE enrichplot fgsea ggtree treeio phyloseq dada2 minfi tximport
tximeta EnhancedVolcano apeglm`)

// common are base R functions called everywhere; no package defines one of these
// for its files to be linked by, so they are not recorded as calls.
var common = set(`c list length paste paste0 is.null stop warning message print cat format
names seq_len seq_along vapply sapply lapply rep invisible identical is.na nchar sprintf
character logical integer numeric vector unlist match.arg missing inherits stopifnot
structure class attr setdiff union intersect unique sort order rev which any all sum max min
abs round mean function return data.frame matrix nrow ncol cbind rbind as.character
as.numeric as.integer as.logical is.character is.numeric is.function is.list tryCatch
on.exit substr substring gsub sub grepl regmatches strsplit toupper tolower trimws file.path
basename dirname file.exists exists get assign environment new.env emptyenv globalenv
Sys.getenv Sys.setenv Sys.time library require requireNamespace quote bquote eval
substitute deparse do.call Recall mapply Map Reduce Filter vapply is.environment
seq max min range rownames colnames dim nlevels levels factor table rapply suppressWarnings
suppressMessages signalCondition conditionMessage conditionCall simpleError simpleCondition
startsWith endsWith encodeString enc2utf8 utf8ToInt intToUtf8 isTRUE isFALSE xor ifelse
setNames modifyList head tail str t mode typeof is.element duplicated anyNA match
vapply Negate identity nargs sys.call sys.function match.call parent.frame
UseMethod NextMethod standardGeneric new validity setClass setGeneric setMethod
setRefClass R6Class source sys.source here p_load use tar_source loadNamespace
suppressPackageStartupMessages local options signature representation`)

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}
