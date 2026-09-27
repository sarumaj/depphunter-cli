Gem::Specification.new do |s|
  s.name        = "mylib"
  s.require_paths = ["lib"]
  s.add_runtime_dependency("rack", [">= 2.0", "< 4"])
  s.add_dependency "rspec-core", "= 3.12.2"
end
