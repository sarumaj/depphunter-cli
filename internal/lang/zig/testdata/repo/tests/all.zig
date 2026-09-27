const std = @import("std");
const shop = @import("shop");
const root = @import("root");

test {
    std.testing.refAllDecls(shop);
}
