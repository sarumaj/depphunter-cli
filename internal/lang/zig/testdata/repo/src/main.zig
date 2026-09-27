const std = @import("std");
const builtin = @import("builtin");
const shop = @import("shop");
const known_folders = @import("known-folders");
const vaxis = @import("vaxis");
const config = @import("config");
const utils = @import("utils");
const model = @import("model");
const tracy = @import("tracy");
const zf = @import("zf");
const args = @import("cli/args.zig");
const gone = @import("missing.zig");
const nowhere = @import("nowhere");

const banner = @embedFile("assets/banner.txt");

const c = @cImport({
    @cInclude("stdio.h");
    @cInclude("shop.h");
});

pub fn main() !void {
    std.debug.print("{s}", .{banner});
}
