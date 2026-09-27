require_relative "../billing"

module Billing
  autoload :Tax, "billing/tax"

  class Invoice
    def total; end
  end
end
