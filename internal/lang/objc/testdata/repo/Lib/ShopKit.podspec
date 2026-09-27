Pod::Spec.new do |s|
  s.name        = "ShopKit"
  s.version     = "2.0.0"
  s.module_name = "Shop"
  s.default_subspec = 'Core'

  s.subspec 'Core' do |core|
    core.source_files = 'Sources/*.{h,m}'
    core.dependency 'AFNetworking', '~> 4.0'
    core.dependency "Mantle", "2.2.0"
  end

  s.subspec 'Promises' do |p|
    p.dependency 'ShopKit/Core'
    p.dependency 'PromiseKit/CorePromise', '>= 6.0', '< 7.0'
  end
end
