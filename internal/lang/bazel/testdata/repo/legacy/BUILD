load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "legacy",
    srcs = ["legacy.go"],
    deps = [
        "@com_github_google_uuid//:uuid",
        "@com_google_absl//absl/strings",
        "@com_github_gflags_gflags//:gflags",
        "@floating//:lib",
        "@vendored//:lib",
        "@zlib",
        "@unpinned//:x",
        "@maven//:com_squareup_okhttp3_okhttp",
        "@legacy//:legacy.go",
    ],
)
