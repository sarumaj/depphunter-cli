require "rack"
require "rack/utils"
require "rspec/core"
require "set"
require "digest/sha2"
require "net/http/persistent"
require "mylib/version"
require File.join(File.dirname(__FILE__), "mylib", "helpers")
require_relative "mylib/missing"
require some_variable
gem "rack"

module Mylib
  class Error < StandardError; end
  def self.root; end
end
