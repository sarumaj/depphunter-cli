module Shop
  module MoneyFormat
    def self.call(value)
      Money.new(value)
    end
  end
end
