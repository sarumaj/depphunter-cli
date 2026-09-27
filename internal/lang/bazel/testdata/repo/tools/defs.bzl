"""Macros of the shop."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load(":private.bzl", _helper = "helper")

ShopInfo = provider(fields = ["name"])

VERSION = "1.0"

def _shop_rule_impl(ctx):
    return [ShopInfo(name = ctx.label.name)]

shop_rule = rule(
    implementation = _shop_rule_impl,
    attrs = {"dep": attr.label(default = Label("//lib:helpers"))},
)

def shop_macro(name, **kwargs):
    """Declares a library and a rule."""
    native.cc_library(name = name, **kwargs)
    shop_rule(name = name + "_rule")
    _helper(paths.join("a", "b"))
