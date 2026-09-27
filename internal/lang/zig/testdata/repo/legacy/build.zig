const std = @import("std");

// Zig 0.11's build API: LazyPath .path fields, CompileStep.addModule and
// .dependencies in module options.
pub fn build(b: *std.Build) void {
    const helper = b.createModule(.{ .source_file = .{ .path = "helper.zig" } });
    const exe = b.addExecutable(.{
        .name = "old",
        .root_source_file = .{ .path = "main.zig" },
    });
    exe.addModule("helper", helper);
    const fmt = b.createModule(.{
        .source_file = .{ .path = "fmt.zig" },
        .dependencies = &.{.{ .name = "helper", .module = helper }},
    });
    exe.addModule("fmt", fmt);
}
