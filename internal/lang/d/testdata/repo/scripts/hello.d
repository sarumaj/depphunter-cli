#!/usr/bin/env dub
/+ dub.sdl:
	name "hello"
	dependency "scriptlike" version="~>0.10.3"
+/
import scriptlike;
import std.stdio;

void main() { writeln("hello"); }
