Gem::Specification.new do |spec|
  spec.name = "billing"
  spec.version = "0.1.0"
  spec.files = Dir["lib/**/*.rb"]

  spec.add_dependency "money", ">= 6"
  spec.add_development_dependency "minitest", "~> 5.0"
end
