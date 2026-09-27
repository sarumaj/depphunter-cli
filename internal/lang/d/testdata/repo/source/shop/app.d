/++
 + The shop's entry point.
 +/
module shop.app;

import std.stdio;
import core.thread : Thread;
static import object;
public import shop.cart, shop.models;
import fmt = std.format;
import std.algorithm : map, filter;
import etc.c.zlib;
import vibe.http.server;
import vibe.core.log;
import dyaml;
import mir.ndslice;
import mir.math.common;
import widgets.button;
import fancy.thing;
import unit_threaded;
import arsd.dom;
import requests;
import eventcore.core;
import commands;
import nothere.x;
import shop.gone;

/+ a nested /+ comment +/ import fake.one; +/
enum code = q{ import fake.two; void f() { string s = "}"; } };
string s1 = "import fake.three;";
string s2 = `import fake.four;`;
string s3 = q"(import (fake.five);)";
string s4 = q"EOS
import fake.six;
EOS";
char quote = '"';
// import fake.seven;
/* import fake.eight; */
string s5 = r"C:\path\" ~ x"0A";

void main()
{
	import std.conv : to;
	mixin(import("banner.txt"));
	auto missing = import("missing.txt");
	writeln(to!string(1));
}
