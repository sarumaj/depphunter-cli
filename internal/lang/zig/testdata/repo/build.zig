const std = @import("std");
const utils = @import("utils");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Exported to packages depending on this one.
    const shop_mod = b.addModule("shop", .{
        .root_source_file = b.path("src/shop.zig"),
        .target = target,
    });

    const options = b.addOptions();
    options.addOption(bool, "fast", true);

    const known_folders = b.dependency("known_folders", .{}).module("known-folders");
    const vaxis_dep = b.dependency("vaxis", .{ .target = target });
    const utils_dep = b.dependency("utils", .{});

    const exe_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "shop", .module = shop_mod },
            .{ .name = "known-folders", .module = known_folders },
            .{ .name = "vaxis", .module = vaxis_dep.module("vaxis") },
            .{ .name = "config", .module = options.createModule() },
        },
    });
    exe_mod.addImport("utils", utils_dep.module("utils"));
    exe_mod.addImport("model", modelModule(b));
    exe_mod.addCSourceFile(.{ .file = b.path("src/native.c") });
    exe_mod.addIncludePath(b.path("include"));

    if (b.lazyDependency("tracy", .{})) |tracy| {
        exe_mod.addImport("tracy", tracy.module("tracy"));
    }

    const exe = b.addExecutable(.{ .name = "shop", .root_module = exe_mod });
    b.installArtifact(exe);

    const tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/all.zig"),
            .imports = &.{.{ .name = "shop", .module = shop_mod }},
        }),
    });
    _ = tests;
    _ = utils;
}

fn modelModule(b: *std.Build) *std.Build.Module {
    const model = b.createModule(.{ .root_source_file = b.path("src/model.zig") });
    return model;
}
