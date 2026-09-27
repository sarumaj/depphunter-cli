package ruby

import "strings"

// std maps what Ruby itself ships to the library it belongs to: the standard library,
// the default gems (json, psych, net-http: gems RubyGems can upgrade, but installed
// with every Ruby) and the bundled gems (minitest, rake, test-unit: installed with
// Ruby, required without a Gemfile). A key is a require path; a require under a key
// marked with a trailing "/" is part of the same library (digest/md5, json/add/core).
// Where a project declares one of these as a gem, the declaration wins (resolve.go):
// Bundler loads the version it locked.
//
// Implements: REQ-RUBY-004
var std = func() map[string]string {
	m := map[string]string{}
	for _, entry := range strings.Fields(`
		abbrev base64 benchmark bigdecimal/ bundler/ cgi/ continuation coverage csv/ date
		debug/ delegate did_you_mean/ digest/ drb/ English erb/ error_highlight/ etc expect
		fcntl fiber fiddle/ fileutils find forwardable getoptlong io/console/=io-console
		io/nonblock=io-nonblock io/wait=io-wait ipaddr irb/ json/ kconv=nkf logger/ matrix
		minitest/ mkmf monitor mutex_m net/ftp=net-ftp net/http=net-http net/https=net-http
		net/imap=net-imap net/pop=net-pop net/protocol=net-protocol net/smtp=net-smtp nkf
		objspace/ observer open-uri open3 openssl/ optparse/ optionparser=optparse ostruct
		pathname power_assert/ pp prettyprint prime prism/ pstore psych/ pty racc/ rake/
		rbconfig/ rbs/ rdoc/ readline reline/ resolv resolv-replace rinda/ ripper/ rss/
		rubygems/ securerandom set shellwords singleton socket stringio strscan
		syntax_suggest/ syslog tempfile test/unit/=test-unit thread time timeout tmpdir
		tsort typeprof/ un unicode_normalize/ uri/ weakref win32ole yaml/=psych zlib
		random/formatter=random-formatter ruby2_keywords objspace/ enumerator rational
		complex io/console/size=io-console
	`) {
		key, pkg, _ := strings.Cut(entry, "=")
		name := strings.TrimSuffix(key, "/")
		if pkg == "" {
			pkg = strings.ToLower(name)
		}
		m[name] = pkg
		if strings.HasSuffix(key, "/") {
			m[name+"/"] = pkg
		}
	}
	return m
}()

// stdLibrary finds the library of Ruby's own a require path belongs to: the path
// itself, or a library that owns everything under it.
func stdLibrary(p string) (string, bool) {
	if pkg, ok := std[p]; ok {
		return pkg, true
	}
	for i := strings.LastIndex(p, "/"); i > 0; i = strings.LastIndex(p[:i], "/") {
		if pkg, ok := std[p[:i]+"/"]; ok {
			return pkg, true
		}
	}
	return "", false
}

// aliases are the require paths whose gem is not named after them: Rails' frameworks
// (action_controller is actionpack), railties behind require "rails", and a few more.
// Several names mean "the first one the project declares, or else the first".
//
// Implements: REQ-RUBY-006
var aliases = map[string][]string{
	"rails":               {"railties", "rails"},
	"abstract_controller": {"actionpack"},
	"action_controller":   {"actionpack"},
	"action_dispatch":     {"actionpack"},
	"action_view":         {"actionview"},
	"action_mailer":       {"actionmailer"},
	"action_cable":        {"actioncable"},
	"action_text":         {"actiontext"},
	"action_mailbox":      {"actionmailbox"},
	"active_job":          {"activejob"},
	"active_model":        {"activemodel"},
	"active_record":       {"activerecord"},
	"active_storage":      {"activestorage"},
	"active_support":      {"activesupport"},
	"concurrent":          {"concurrent-ruby"},
	"sprockets/railtie":   {"sprockets-rails"},
	"html/pipeline":       {"html-pipeline"},
	"google/protobuf":     {"google-protobuf"},
	"zip":                 {"rubyzip"},
	"openai":              {"ruby-openai"},
}
