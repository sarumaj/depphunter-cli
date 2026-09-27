module Shop
  module Catalog
    VERSION = "1.0"

    class Item < Struct.new(:sku)
      attr_reader :price
      LIMIT = 10

      def initialize(sku)
        super
        max = 3
      end

      def self.build; end

      class << self
        def import; end
      end

      private def secret; end
    end
  end
end

class Shop::Catalog::Price
  def to_s; end
end

def helper; end
