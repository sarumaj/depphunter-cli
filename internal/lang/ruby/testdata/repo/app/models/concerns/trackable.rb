module Trackable
  extend ActiveSupport::Concern

  class_methods do
    def tracked?
      true
    end
  end
end
