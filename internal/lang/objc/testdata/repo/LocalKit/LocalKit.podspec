Pod::Spec.new do |s|
  s.name         = 'LocalKit'
  s.version      = '0.1.0'
  s.source       = { :git => 'https://github.com/acme/LocalKit.git', :tag => s.version.to_s }
  s.source_files = 'Sources/**/*.{h,m}'
  s.dependency 'AFNetworking/NSURLSession', '~> 4.0'
end
