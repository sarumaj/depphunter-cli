## The shop. A doc comment hides a fake import:
## import fakedoc
#[ A block comment
  import fakeblock
  #[ nested ]# import stillcomment
]#
##[ A doc block comment
import fakedocblock
]##
import std/[os, strutils], tables, json
import pkg/jester
import chronos, chronos/asyncloop
import widgets/button
import stew/byteutils, results, zippy, pixie
import vault/client
import httputils, sdl2
import winim/lean
import ./shop/cart, shop/models/user
import "shop/private/util.nim"
import
  vlib,
  generated, helper
import missing/thing
import ./nothere
import db_sqlite
import std/sha1 as sha
import strutils except split, join
from sequtils import map, filter
include shop/inc
export tables

let s = "import fakestring \" still"
let r = r"import fakeraw "" still"
let t = """
import faketriple
"""
let f = fmt"import {s}"
let c = '"'
let n = 1'i32

when defined(windows):
  import winlean
  proc native*() = discard
elif defined(js): import jsffi
else:
  import posix

type
  Money* = distinct int
  Item* = object
    name*: string
    price: Money
  Kind {.pure.} = enum
    kA, kB
  Shape* = ref object of RootObj
  Comparable* = concept x
    x < x is bool
  Callback = proc (x: int): int
  Pair*[T] = tuple[a, b: T]

type Cart* = object
  items: seq[Item]

const Version* = "1.0"
const
  A* = 1
  B = 2
var counter*, grand: int
let (x, y) = (1, 2)
var tls {.threadvar.}: int

proc total*(items: seq[Item]): Money =
  proc inner() = discard
  import notatoplevel
  result = Money(0)

func `$`*(m: Money): string = $int(m)
method draw*(s: Shape) {.base.} = discard
iterator items*(c: Cart): Item = discard
converter toInt(m: Money): int = int(m)
template check*(x: untyped) = discard
macro route*(body: untyped): untyped = body
proc forward(x: int)
proc (x: int) = discard

when isMainModule:
  proc main() = discard
  main()
