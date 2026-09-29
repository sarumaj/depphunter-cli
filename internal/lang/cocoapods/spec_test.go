package cocoapods

import (
	"reflect"
	"testing"
)

// A Ruby podspec is read as the CDN's podspec.json is shaped: the root spec's
// dependencies (a platform's included), each top-level subspec's with those of
// the subspecs nested in it, and the default subspecs; a test or app spec's
// dependencies, and anything nested in one, are left out.
//
// Verifies: REQ-SUP-074
func TestReadSpec(t *testing.T) {
	source := `Pod::Spec.new do |s|
  s.name         = 'AcmeKit'
  s.version      = '1.2.0'
  s.default_subspecs = %w[Core]
  s.dependency 'AFNetworking', '~> 4.0'
  s.ios.dependency 'Mantle', '= 2.2.0' # only on iOS
  s.subspec 'Core' do |core|
    core.dependency 'AcmeKit/Base'
    core.subspec 'Deep' do |deep|
      deep.dependency 'SDWebImage', '>= 5.0',
        '< 6.0'
    end
  end
  s.subspec "Extras" do |extras|
    extras.dependency "PromiseKit"
  end
  s.test_spec 'Tests' do |test_spec|
    test_spec.dependency 'OCMock'
    test_spec.subspec 'Nested' do |nested|
      nested.dependency 'Quick'
    end
  end
  s.app_spec 'App' do |app|
    app.dependency 'Nimble'
  end
end
`
	want := Spec{
		Name:            "AcmeKit",
		Dependencies:    map[string][]string{"AFNetworking": {"~> 4.0"}, "Mantle": {"= 2.2.0"}},
		DefaultSubspecs: []string{"Core"},
		Subspecs: []Spec{
			{Name: "Core", Dependencies: map[string][]string{"AcmeKit/Base": nil, "SDWebImage": {">= 5.0", "< 6.0"}}},
			{Name: "Extras", Dependencies: map[string][]string{"PromiseKit": nil}},
		},
	}
	if got := ReadSpec([]byte(source)); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	for value, want := range map[string][]string{
		`'Core'`:            {"Core"},
		`['Core', "UI"]`:    {"Core", "UI"},
		`%w(Core UI)`:       {"Core", "UI"},
		`:none`:             nil,
		`%w[Core] # a note`: {"Core"},
	} {
		if got := words(value); !reflect.DeepEqual(got, want) {
			t.Errorf("default_subspecs = %s: got %v, want %v", value, got, want)
		}
	}
	if got := ReadSpec([]byte("s.dependency 'Orphan'\nnot ruby at all (((")); len(got.Dependencies) != 0 || got.Name != "" {
		t.Errorf("no Pod::Spec block: got %+v", got)
	}
}
