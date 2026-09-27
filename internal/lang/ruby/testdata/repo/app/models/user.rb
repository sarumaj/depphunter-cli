class User < ApplicationRecord
  include Trackable

  attr_accessor :nickname, :locale
  ROLES = %w[admin member].freeze

  def self.admins
    Admin::Role.where(name: "admin")
  end

  def display_name
    Shop::MoneyFormat.call(nickname)
  end
end
