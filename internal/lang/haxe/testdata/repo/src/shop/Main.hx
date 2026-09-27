package shop;

import shop.model.Cart;
import shop.model.Cart.CartItem;
import shop.util.*;
import shop.util.Money.*;
import haxe.Json;
import haxe.ds.StringMap;
import sys.io.File as F;
import StringTools;
using Lambda;
using shop.util.Strings;
import openfl.display.Sprite;
import tink.core.Future;
import thx.Arrays;
import format.png.Reader;
import gfx.Canvas;
import js.node.Fs;
import haxe.ui.Toolkit;
import mystery.Thing;
import shop.Gone;
import std.Math;
import Config;
#if js
import js.Browser;
#elseif (cpp && !debug)
import cpp.Lib;
#else
import sys.FileSystem;
#end

/* import fake.Comment; */
// import fake.Line;

@:keep
class Main extends Sprite {
	static var banner = 'import fake.Single ${shop.util.Strings.upper("}")} $name';
	static var re = ~/import fake\.Regex/g;
	static var s = "import fake.Double; \"quoted\"";

	public function new() {
		super();
		var j = haxe.crypto.Md5.encode("x");
		var c = new shop.model.Cart();
		trace(flixel.FlxG.width);
		var x = key.ID;
	}

	static function main() {
		new Main();
	}

	@:noCompletion inline function helper():Void {}
}

#if flash
class Extra extends flash.display.Sprite {
#else
class Extra {
#end
	public function run() {}
}

interface Service {
	function call(x:Int):Void;
	function stop():Void;
}

enum Color {
	Red;
	Rgb(r:Int, g:Int, b:Int);
}

enum abstract Level(Int) from Int to Int {
	var Low = 1;
	public function isHigh():Bool return this > 1;
}

abstract Meters(Float) {
	public inline function new(v:Float) this = v;
	@:op(A + B) function add(b:Meters):Meters return new Meters(this + (b : Float));
}

typedef Options = {
	var verbose:Bool;
	@:optional var name:String;
}

abstract class Base<T:{}> {
	abstract function run():Void;
}

function helperAtModuleLevel() {}

final VERSION = "1.0";
