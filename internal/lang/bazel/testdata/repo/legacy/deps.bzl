load("@bazel_gazelle//:deps.bzl", "go_repository")
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def legacy_deps():
    go_repository(
        name = "com_github_google_uuid",
        importpath = "github.com/google/uuid",
        version = "v1.6.0",
        sum = "h1:fake",
    )
    maybe(
        http_archive,
        name = "com_google_absl",
        urls = ["https://github.com/abseil/abseil-cpp/archive/4a2c63365eff8823a5221db86ef490e828306f9d.tar.gz"],
    )
