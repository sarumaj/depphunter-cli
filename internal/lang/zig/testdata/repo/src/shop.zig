//! The shop's module.
const std = @import("std");
const root = @import("root");
pub const Allocator = std.mem.Allocator;

pub const Cart = struct {
    items: std.ArrayList(Item) = .empty,
    total: u64 = 0,

    pub const Item = struct {
        sku: []const u8,
        qty: u32,

        pub fn price(self: Item) u64 {
            return self.qty;
        }
    };

    const limit = 10;

    pub fn add(self: *Cart, item: Item) error{Full}!void {
        _ = self;
        _ = item;
    }

    fn nested() void {
        const Local = struct {
            fn hidden() void {}
        };
        _ = Local;
    }
};

pub const Color = enum(u8) { red, green, _ };

const Shape = union(enum) {
    circle: f32,
    square: f32,

    pub fn area(s: Shape) f32 {
        return switch (s) {
            .circle => |r| r * r,
            .square => |a| a * a,
        };
    }
};

pub const Error = error{ OutOfStock, Closed };
const Handle = opaque {};
const Packed = packed struct(u8) { a: u4, b: u4 };

var counter: u32 = 0;
pub extern "c" fn abs(x: c_int) c_int;
threadlocal var last: ?*Cart = null;

pub fn checkout(cart: *Cart) struct { ok: bool } {
    _ = cart;
    return .{ .ok = true };
}

pub fn Pair(comptime T: type) type {
    return struct { a: T, b: T };
}

comptime {
    _ = checkout;
}

test "cart adds items" {
    try std.testing.expect(true);
}

test checkout {}

const @"weird name" = 1;
