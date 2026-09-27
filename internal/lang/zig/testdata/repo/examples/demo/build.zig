const std = @import("std");

pub fn build(b: *std.Build) void {
    const shop = b.dependency("shop", .{});
    const exe = b.addExecutable(.{
        .name = "demo",
        .root_module = b.createModule(.{ .root_source_file = b.path("main.zig") }),
    });
    exe.root_module.addImport("shop", shop.module("shop"));
}
