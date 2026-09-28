package ruby

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// check compares every analyzed file of a fixture with the imports and symbols it
// should have; a file missing from want must have none.
func check(t *testing.T, root string, imports map[string]map[string]lang.Target, symbols map[string]map[string]string) {
	t.Helper()
	res := langtest.Analyze(t, Plugin{}, root)
	for file, r := range res {
		t.Run(file, func(t *testing.T) {
			langtest.CheckImports(t, r, imports[file])
			want := symbols[file]
			if want == nil {
				want = map[string]string{}
			}
			langtest.CheckSymbols(t, r, want)
		})
	}
	for file := range imports {
		if res[file] == nil {
			t.Errorf("%s: not analyzed", file)
		}
	}
}

// A Rails application with a Gemfile, its lock and a gem kept in the repository:
// requires on the load path, require_relative and load, Ruby's library, gems by the
// lock, the Gemfile's own declarations, and constants Zeitwerk would autoload.
//
// Verifies: REQ-RUBY-001, REQ-RUBY-002, REQ-RUBY-003, REQ-RUBY-004, REQ-RUBY-005
// Verifies: REQ-RUBY-006, REQ-RUBY-007, REQ-RUBY-008, REQ-RUBY-009, REQ-RUBY-010, REQ-FND-026
func TestRailsApplication(t *testing.T) {
	check(t, "testdata/repo", map[string]map[string]lang.Target{
		"Gemfile": {
			`gem "rails", "~> 7.1.0"`: {Ecosystem: ecoGems, Package: "rails", Version: "7.1.2", Requested: "~> 7.1.0", Pinned: true},
			`gem "pg", "1.5.4"`:       {Ecosystem: ecoGems, Package: "pg", Version: "1.5.4", Pinned: true},
			`gem "puma", ">= 5.0"`:    {Ecosystem: ecoGems, Package: "puma", Version: "6.4.0", Requested: ">= 5.0", Pinned: true},
			`gem "sidekiq"`:           {Ecosystem: ecoGems, Package: "sidekiq", Version: "7.2.0", Pinned: true},
			`gem "devise", github: "heartcombo/devise", ref: "0123456789abcdef0123456789abcdef01234567"`: {Ecosystem: ecoGems, Package: "devise", Version: "4.9.3", Pinned: true, Origin: "https://github.com/heartcombo/devise.git", Git: "https://github.com/heartcombo/devise.git#0123456789abcdef0123456789abcdef01234567"},
			`gem "billing", path: "gems/billing"`:                                                        {Local: "gems/billing/billing.gemspec"},
			`gem "rspec-rails", "~> 6.0", require: false`:                                                {Ecosystem: ecoGems, Package: "rspec-rails", Version: "6.1.0", Requested: "~> 6.0", Pinned: true},
			`gem "acme-auth"`: {Ecosystem: ecoGems, Package: "acme-auth", Version: "2.0.1", Pinned: true},
		},
		"Rakefile": {
			`require_relative "config/application"`: {Local: "config/application.rb"},
			`load "lib/tasks/seed.rake"`:            {Local: "lib/tasks/seed.rake"},
			"Rails":                                 {},
		},
		"app/controllers/admin/users_controller.rb": {
			"ApplicationController":        {Local: "app/controllers/application_controller.rb"},
			"User":                         {Local: "app/models/user.rb"},
			"Role":                         {Local: "app/models/admin/role.rb"},
			"::Admin::Missing":             {},
			"ActiveRecord::RecordNotFound": {},
		},
		"app/controllers/application_controller.rb": {
			"ActionController::Base": {},
		},
		"app/models/admin/role.rb": {
			"ApplicationRecord": {Local: "app/models/application_record.rb"},
		},
		"app/models/application_record.rb": {
			"ActiveRecord::Base": {},
		},
		"app/models/concerns/trackable.rb": {
			"ActiveSupport::Concern": {},
		},
		"app/models/user.rb": {
			"ApplicationRecord": {Local: "app/models/application_record.rb"},
			"Trackable":         {Local: "app/models/concerns/trackable.rb"},
			"Admin::Role":       {Local: "app/models/admin/role.rb"},
			"Shop::MoneyFormat": {Local: "lib/shop/money_format.rb"},
		},
		"config.ru": {
			`require_relative "config/environment"`: {Local: "config/environment.rb"},
			"Rails":                                 {},
		},
		"config/application.rb": {
			`require_relative "boot"`:             {Local: "config/boot.rb"},
			`require "rails/all"`:                 {Ecosystem: ecoGems, Package: "railties", Version: "7.1.2", Pinned: true},
			`require "action_controller/railtie"`: {Ecosystem: ecoGems, Package: "actionpack", Version: "7.1.2", Pinned: true},
			"Rails::Application":                  {},
		},
		"config/boot.rb": {
			`require "bundler/setup"`:  {Ecosystem: ecoStd, Package: "bundler"},
			`require "bootsnap/setup"`: {Ecosystem: ecoGems, Package: "bootsnap", Unresolved: true},
		},
		"config/environment.rb": {
			`require_relative "application"`: {Local: "config/application.rb"},
			"Rails":                          {},
		},
		"gems/billing/billing.gemspec": {
			"Gem::Specification":                                   {},
			`spec.add_dependency "money", ">= 6"`:                  {Ecosystem: ecoGems, Package: "money", Version: "6.16.0", Pinned: true},
			`spec.add_development_dependency "minitest", "~> 5.0"`: {Ecosystem: ecoGems, Package: "minitest", Version: "~> 5.0"},
		},
		"gems/billing/lib/billing.rb": {
			`require "billing/invoice"`: {Local: "gems/billing/lib/billing/invoice.rb"},
			`require "json"`:            {Ecosystem: ecoStd, Package: "json"},
			`require "money"`:           {Ecosystem: ecoGems, Package: "money", Version: "6.16.0", Pinned: true},
		},
		"gems/billing/lib/billing/invoice.rb": {
			`require_relative "../billing"`: {Local: "gems/billing/lib/billing.rb"},
			`autoload :Tax, "billing/tax"`:  {Local: "gems/billing/lib/billing/tax.rb"},
		},
		"gems/billing/test/invoice_test.rb": {
			`require "test_helper"`: {Local: "gems/billing/test/test_helper.rb"},
			"Minitest::Test":        {},
		},
		"gems/billing/test/test_helper.rb": {
			`require "minitest/autorun"`: {Ecosystem: ecoGems, Package: "minitest", Version: "~> 5.0"},
			`require "billing"`:          {Local: "gems/billing/lib/billing.rb"},
		},
		"lib/shop/catalog.rb": {
			"Struct": {},
		},
		"lib/shop/money_format.rb": {
			"Money": {},
		},
		"lib/tasks/seed.rake": {
			`require "csv"`: {Ecosystem: ecoStd, Package: "csv"},
			"CSV":           {},
			"User":          {Local: "app/models/user.rb"},
		},
		"spec/models/user_spec.rb": {
			`require "rails_helper"`: {Local: "spec/rails_helper.rb"},
			"RSpec":                  {},
			"User":                   {Local: "app/models/user.rb"},
		},
		"spec/rails_helper.rb": {
			"File": {},
			`require File.expand_path("../config/environment", __dir__)`: {Local: "config/environment.rb"},
			`require "rspec/rails"`: {Ecosystem: ecoGems, Package: "rspec-rails", Version: "6.1.0", Requested: "~> 6.0", Pinned: true},
			`require "net/http"`:    {Ecosystem: ecoStd, Package: "net-http"},
			`require "yaml"`:        {Ecosystem: ecoGems, Package: "psych", Version: "5.1.1", Pinned: true},
		},
	}, map[string]map[string]string{
		"app/controllers/admin/users_controller.rb": {"Admin": "module", "Admin::UsersController": "class", "Admin::UsersController.index": "method"},
		"app/controllers/application_controller.rb": {"ApplicationController": "class"},
		"app/models/admin/role.rb":                  {"Admin": "module", "Admin::Role": "class"},
		"app/models/application_record.rb":          {"ApplicationRecord": "class"},
		"app/models/concerns/trackable.rb":          {"Trackable": "module", "Trackable.tracked?": "method"},
		"app/models/user.rb":                        {"User": "class", "User.nickname": "attr", "User.locale": "attr", "User::ROLES": "const", "User.admins": "method", "User.display_name": "method"},
		"config/application.rb":                     {"Shop": "module", "Shop::Application": "class"},
		"gems/billing/lib/billing.rb":               {"Billing": "module", "Billing::VERSION": "const"},
		"gems/billing/lib/billing/invoice.rb":       {"Billing": "module", "Billing::Invoice": "class", "Billing::Invoice.total": "method"},
		"gems/billing/lib/billing/tax.rb":           {"Billing": "module", "Billing::Tax": "class"},
		"gems/billing/test/invoice_test.rb":         {"InvoiceTest": "class", "InvoiceTest.test_total": "method"},
		"lib/shop/catalog.rb":                       {"Shop": "module", "Shop::Catalog": "module", "Shop::Catalog::VERSION": "const", "Shop::Catalog::Item": "class", "Shop::Catalog::Item.price": "attr", "Shop::Catalog::Item::LIMIT": "const", "Shop::Catalog::Item.initialize": "method", "Shop::Catalog::Item.build": "method", "Shop::Catalog::Item.import": "method", "Shop::Catalog::Item.secret": "method", "Shop::Catalog::Price": "class", "Shop::Catalog::Price.to_s": "method", "helper": "func"},
		"lib/shop/money_format.rb":                  {"Shop": "module", "Shop::MoneyFormat": "module", "Shop::MoneyFormat.call": "method"},
	})
}

// A gem library without a lock: its gemspec is all there is, a Gemfile's gemspec
// directive points at it, and a require nothing declares is named after its path.
//
// Verifies: REQ-RUBY-002, REQ-RUBY-004, REQ-RUBY-005, REQ-RUBY-006, REQ-RUBY-007, REQ-RUBY-009
func TestGemLibrary(t *testing.T) {
	check(t, "testdata/gemlib", map[string]map[string]lang.Target{
		"Gemfile": {
			"gemspec":              {Local: "mylib.gemspec"},
			`gem "rake", "13.1.0"`: {Ecosystem: ecoGems, Package: "rake", Version: "13.1.0", Pinned: true},
		},
		"lib/mylib.rb": {
			`require "rack"`:                {Ecosystem: ecoGems, Package: "rack", Version: ">= 2.0, < 4"},
			`require "rack/utils"`:          {Ecosystem: ecoGems, Package: "rack", Version: ">= 2.0, < 4"},
			`require "rspec/core"`:          {Ecosystem: ecoGems, Package: "rspec-core", Version: "3.12.2", Pinned: true},
			`require "set"`:                 {Ecosystem: ecoStd, Package: "set"},
			`require "digest/sha2"`:         {Ecosystem: ecoStd, Package: "digest"},
			`require "net/http/persistent"`: {Ecosystem: ecoGems, Package: "net-http", Unresolved: true},
			`require "mylib/version"`:       {Local: "lib/mylib/version.rb"},
			"File":                          {},
			`require File.join(File.dirname(__FILE__), "mylib", "helpers")`: {Local: "lib/mylib/helpers.rb"},
			`require_relative "mylib/missing"`:                              {},
			`gem "rack"`:                                                    {Ecosystem: ecoGems, Package: "rack", Version: ">= 2.0, < 4"},
			"StandardError":                                                 {},
		},
		"mylib.gemspec": {
			"Gem::Specification": {},
			`s.add_runtime_dependency("rack", [">= 2.0", "< 4"])`: {Ecosystem: ecoGems, Package: "rack", Version: ">= 2.0, < 4"},
			`s.add_dependency "rspec-core", "= 3.12.2"`:           {Ecosystem: ecoGems, Package: "rspec-core", Version: "3.12.2", Pinned: true},
		},
	}, map[string]map[string]string{
		"lib/mylib.rb":         {"Mylib": "module", "Mylib::Error": "class", "Mylib.root": "method"},
		"lib/mylib/helpers.rb": {"Mylib": "module", "Mylib::Helpers": "module"},
		"lib/mylib/version.rb": {"Mylib": "module", "Mylib::VERSION": "const"},
	})
}

// Verifies: REQ-RUBY-001
func TestClaims(t *testing.T) {
	for _, p := range []string{
		"lib/a.rb", "lib/tasks/db.rake", "a.gemspec", "config.ru", "Gemfile", "Rakefile",
		"sub/Guardfile", "deploy/Capfile", "B.RB",
	} {
		if !(Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: not claimed", p)
		}
	}
	for _, p := range []string{"Gemfile.lock", "gems.locked", "a.erb", "rb", "gemfile", "a.rbs"} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: claimed", p)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "a.rb", Binary: true}) {
		t.Error("a binary file was claimed")
	}
}

// Verifies: REQ-RUBY-009
func TestPinned(t *testing.T) {
	for req, want := range map[string]bool{
		"1.2.3": true, "= 1.2.3": true, "=1.2.3": true, "7.1.0.rc1": true, "= 7.1.0.beta2": true,
		"~> 1.2": false, ">= 1.0": false, "> 1": false, "< 2": false, "!= 1.2.3": false,
		">= 1.0, < 2": false, "": false, "1.2.x": false,
	} {
		if got := pinned(req); got != want {
			t.Errorf("pinned(%q) = %v, want %v", req, got, want)
		}
	}
}

// Verifies: REQ-RUBY-002
func TestEvalPath(t *testing.T) {
	for expr, want := range map[string]string{
		`"a/b"`: "a/b",
		`'a/b'`: "a/b",
		`File.expand_path("../config/environment", __dir__)`:       "__DIR__/../config/environment",
		`File.expand_path('../../config/environment', __FILE__)`:   "__DIR__/_/../../config/environment",
		`File.join(File.dirname(__FILE__), "lib", "x")`:            "__DIR__/lib/x",
		`File.dirname(__FILE__) + "/x"`:                            "__DIR__/x",
		`__dir__ + '/x'`:                                           "__DIR__/x",
		`"#{__dir__}/x"`:                                           "__DIR__/x",
		`::File.expand_path(File.join(__dir__, "..", "lib", "a"))`: "__DIR__/../lib/a",
		`name`:                 "",
		`"#{name}/x"`:          "",
		`"a" + name`:           "",
		`Rails.root.join("x")`: "",
	} {
		got, ok := evalPath(expr)
		if want == "" && ok || want != "" && got != want {
			t.Errorf("evalPath(%s) = %q, %v; want %q", expr, got, ok, want)
		}
	}
}

// The lock's graph answers --resolve-depth: a locked gem's dependencies at their
// locked versions, the requirement kept when it is not the version itself; a gem
// locked for two platforms is one gem.
//
// Verifies: REQ-RUBY-008, REQ-RUBY-011
func TestLockGraph(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	got := r.Dependencies(lang.Target{Ecosystem: ecoGems, Package: "railties", Version: "7.1.2"})
	want := []lang.Target{
		{Ecosystem: ecoGems, Package: "actionpack", Version: "7.1.2", Pinned: true},
		{Ecosystem: ecoGems, Package: "activesupport", Version: "7.1.2", Pinned: true},
		{Ecosystem: ecoGems, Package: "psych", Version: "5.1.1", Pinned: true},
	}
	if !equal(got, want) {
		t.Errorf("railties: got %+v, want %+v", got, want)
	}
	got = r.Dependencies(lang.Target{Ecosystem: ecoGems, Package: "nokogiri"})
	want = []lang.Target{{Ecosystem: ecoGems, Package: "racc", Version: "1.7.3", Requested: "~> 1.4", Pinned: true}}
	if !equal(got, want) {
		t.Errorf("nokogiri: got %+v, want %+v", got, want)
	}
	// A dependency the lock does not hold is its requirement.
	got = r.Dependencies(lang.Target{Ecosystem: ecoGems, Package: "devise", Version: "4.9.3"})
	want = []lang.Target{{Ecosystem: ecoGems, Package: "railties", Version: "7.1.2", Requested: ">= 4.1.0", Pinned: true}}
	if !equal(got, want) {
		t.Errorf("devise: got %+v, want %+v", got, want)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecoStd, Package: "json"}); got != nil {
		t.Errorf("a standard library got dependencies: %+v", got)
	}
}

func equal(a, b []lang.Target) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Verifies: REQ-RUBY-007
func TestReadGemfile(t *testing.T) {
	src := `source "https://rubygems.org"
gemspec path: "core"
gem "rails", "~> 7.1", ">= 7.1.2" # comment, "9"
gem("pg")
path "engines" do
  gem "admin"
end
git "https://git.example/acme/tools.git", ref: "0123456789abcdef0123456789abcdef01234567" do
  gem "acme-tools"
end
group :test do
  gem "rspec",
    "3.13.0"
end
eval_gemfile "Gemfile.shared"
`
	decls, dirs := readGemfile(src, func(rel string) (string, bool) {
		return "gem 'shared'\n", rel == "Gemfile.shared"
	})
	got := map[string]declaration{}
	for _, d := range decls {
		got[d.name] = *d
	}
	want := map[string]declaration{
		"rails":      {name: "rails", requirement: "~> 7.1, >= 7.1.2"},
		"pg":         {name: "pg"},
		"admin":      {name: "admin", origin: "path:engines"},
		"acme-tools": {name: "acme-tools", origin: "https://git.example/acme/tools.git"},
		"rspec":      {name: "rspec", requirement: "3.13.0"},
		"shared":     {name: "shared"},
	}
	if len(got) != len(want) {
		t.Errorf("got %+v", got)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got %+v, want %+v", name, got[name], w)
		}
	}
	if len(dirs) != 1 || dirs[0] != "core" {
		t.Errorf("gemspec directories %q, want [core]", dirs)
	}
}
