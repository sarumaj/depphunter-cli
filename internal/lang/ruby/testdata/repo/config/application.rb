require_relative "boot"

require "rails/all"
require "action_controller/railtie"

module Shop
  class Application < Rails::Application
    config.autoload_lib(ignore: %w[assets tasks])
  end
end
