# Shop: a small web shop. require "commented" is not read.
require "./shop/*"
require "./shop/models/**"
require "./missing"
require "json"
require "http/client"
require "digest/sha256"
require "c/stdio"
require "kemal"
require "kemal/cli"
require "exception_page"
require "db"
require "sqlite3"
require "markd"
require "radix"
require "crinja"
require "internal/client"
require "widgets"
require "widgets/button"
require "shop/version"
require "nothere/thing"

{% if flag?(:win32) %}
  require "./shop/windows"
{% end %}

module Shop
  VERSION_NAME = "shop #{VERSION}"
  TEMPLATE     = <<-HTML
    require "heredoc"
    class Fake
    end
    HTML
  QUOTED = %(require "percent")
  WORDS  = %w(require class def)
  CHAR   = '"'
  RE     = /require "regex"/

  annotation Audited
  end

  @[Link("ssl")]
  lib LibSSL
    alias SizeT = UInt64
    type Context = Void*

    struct Buffer
      data : UInt8*
    end

    union Value
      i : Int32
    end

    fun ssl_init = OPENSSL_init_ssl(opts : UInt64, settings : Void*) : Int32
    fun ssl_free(ctx : Context)
  end

  enum Status : UInt8
    Open   = 1
    Closed

    def open?
      self == Open
    end
  end

  alias Money = Int64

  record Price, amount : Money, currency : String do
    def to_s(io)
      io << amount
    end
  end

  abstract class Item
    abstract def price : Price

    getter name : String
    property? active : Bool = true
    class_getter count = 0
  end

  class Store < Item
    def initialize(@name : String)
    end

    def price : Price
      return Price.new(0, "EUR") if @name.empty?
      Price.new(1, "EUR")
    end

    def self.open(name) : Store
      new(name)
    end

    def [](i : Int32)
      @items[i]?
    end

    def ==(other : Store)
      name == other.name
    end

    def name=(value : String)
      @name = value
    end

    private def begin
      # a method may be called begin
      range = 1..2
      range.begin if range.end > 1
      label = {class: "x", if: 1}
    end

    macro delegate_all(to)
      def {{to.id}}_name
        {{to}}.name
      end

      def fake_in_macro
      end
    end

    delegate_all store
  end
end

def main
  puts Shop::VERSION
  x = if true
        1
      else
        2
      end
end
