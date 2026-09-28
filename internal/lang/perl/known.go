package perl

import "strings"

// Requirements and imports name modules, but CPAN releases, versions and advisories
// are per distribution, so the map's packages are distributions, named as
// MetaCPAN names them (libwww-perl, Moose, Test-Simple). Online, 02packages or
// MetaCPAN's module endpoint says which distribution provides a module; offline,
// cpanfile.snapshot says it for what Carton installed, and otherwise the name is
// the module's with "::" as "-" - true of most distributions' main modules - except
// where distAliases knows better.

// distAliases maps a module, or with a trailing "::" every module under a
// namespace, to the distribution that provides it where that is not the module's
// own name. An empty value stops a namespace entry (the module is its own
// distribution). The longest matching key wins.
//
// Implements: REQ-PERL-008
var distAliases = map[string]string{
	// libwww-perl and its split-off distributions
	"LWP": "libwww-perl", "LWP::": "libwww-perl", "LWP::Protocol::https": "LWP-Protocol-https",
	"LWP::MediaTypes": "LWP-MediaTypes", "HTTP::Request": "HTTP-Message", "HTTP::Request::Common": "HTTP-Message",
	"HTTP::Response": "HTTP-Message", "HTTP::Headers": "HTTP-Message", "HTTP::Headers::Util": "HTTP-Message",
	"HTTP::Status": "HTTP-Message", "HTTP::Message": "HTTP-Message", "HTTP::Config": "HTTP-Message",
	"HTTP::Cookies": "HTTP-Cookies", "HTTP::Date": "HTTP-Date", "HTTP::Negotiate": "HTTP-Negotiate",
	"URI::": "URI", "HTML::Entities": "HTML-Parser", "HTML::TokeParser": "HTML-Parser",
	"HTML::HeadParser": "HTML-Parser", "HTML::LinkExtor": "HTML-Parser", "HTML::PullParser": "HTML-Parser",
	// frameworks whose modules live in one distribution named otherwise
	"Mojo": "Mojolicious", "Mojo::": "Mojolicious", "Mojolicious::": "Mojolicious", "Mojolicious::Plugin::": "",
	"Mojolicious::Plugin::Config": "Mojolicious", "Mojolicious::Plugin::DefaultHelpers": "Mojolicious",
	"Mojolicious::Plugin::EPLRenderer": "Mojolicious", "Mojolicious::Plugin::EPRenderer": "Mojolicious",
	"Mojolicious::Plugin::HeaderCondition": "Mojolicious", "Mojolicious::Plugin::JSONConfig": "Mojolicious",
	"Mojolicious::Plugin::Mount": "Mojolicious", "Mojolicious::Plugin::NotYAMLConfig": "Mojolicious",
	"Mojolicious::Plugin::TagHelpers": "Mojolicious",
	"Catalyst":                        "Catalyst-Runtime", "Catalyst::Controller": "Catalyst-Runtime", "Catalyst::Model": "Catalyst-Runtime",
	"Catalyst::View": "Catalyst-Runtime", "Catalyst::Test": "Catalyst-Runtime", "Catalyst::Request": "Catalyst-Runtime",
	"Catalyst::Response": "Catalyst-Runtime", "Catalyst::Utils": "Catalyst-Runtime", "Catalyst::Exception": "Catalyst-Runtime",
	"Catalyst::Component": "Catalyst-Runtime", "Catalyst::Action": "Catalyst-Runtime", "Catalyst::Log": "Catalyst-Runtime",
	"Catalyst::Engine": "Catalyst-Runtime", "Catalyst::Runtime": "Catalyst-Runtime", "Catalyst::ScriptRunner": "Catalyst-Runtime",
	"Plack::": "Plack", "Plack::Middleware::": "", "Dancer2::": "Dancer2", "Dancer2::Plugin::": "",
	"Template": "Template-Toolkit", "Template::": "Template-Toolkit",
	"Apache2::": "mod_perl2", "APR::": "mod_perl2", "ModPerl::": "mod_perl2",
	"CGI::Carp": "CGI", "CGI::Cookie": "CGI", "CGI::Util": "CGI", "CGI::Pretty": "CGI", "CGI::Push": "CGI",
	// object systems
	"Moose::": "Moose", "Class::MOP": "Moose", "Class::MOP::": "Moose", "Moo::": "Moo", "Mouse::": "Mouse",
	"Role::Tiny::With": "Role-Tiny", "Types::Standard": "Type-Tiny", "Types::Common": "Type-Tiny",
	"Types::Common::": "Type-Tiny", "Types::TypeTiny": "Type-Tiny", "Type::Utils": "Type-Tiny",
	"Type::Library": "Type-Tiny", "Type::Params": "Type-Tiny", "Type::Tiny::": "Type-Tiny", "Specio::": "Specio",
	"MooseX::Types::Moose": "MooseX-Types",
	// the date and time family: DateTime's own modules, not its plug-in namespaces
	"DateTime::": "DateTime", "DateTime::Format::": "", "DateTime::Event::": "", "DateTime::Calendar::": "",
	"DateTime::TimeZone": "DateTime-TimeZone", "DateTime::TimeZone::": "DateTime-TimeZone",
	"DateTime::Locale": "DateTime-Locale", "DateTime::Locale::": "DateTime-Locale",
	"DateTime::Tiny": "DateTime-Tiny", "DateTime::Set": "DateTime-Set", "DateTime::Span": "DateTime-Set",
	"DateTime::SpanSet": "DateTime-Set",
	// testing: Test::More and Test2's core are Test-Simple; the rest of Test2 is Test2-Suite
	"Test::More": "Test-Simple", "Test::Builder": "Test-Simple", "Test::Builder::": "Test-Simple",
	"Test::Simple": "Test-Simple", "Test::Tester": "Test-Simple", "Test::use::ok": "Test-Simple", "ok": "Test-Simple",
	"Test2::": "Test2-Suite", "Test2::API": "Test-Simple", "Test2::API::": "Test-Simple", "Test2::Event": "Test-Simple",
	"Test2::Event::": "Test-Simple", "Test2::EventFacet::": "Test-Simple", "Test2::Hub": "Test-Simple",
	"Test2::Hub::": "Test-Simple", "Test2::Formatter": "Test-Simple", "Test2::Formatter::": "Test-Simple",
	"Test2::IPC": "Test-Simple", "Test2::IPC::": "Test-Simple", "Test2::Util": "Test-Simple",
	"Test2::Util::HashBase": "Test-Simple", "Test2::Tools::Tiny": "Test-Simple", "Test2::Plugin::": "Test2-Suite",
	"Test::Deep::": "Test-Deep",
	// dual-life core modules shipped in distributions named otherwise
	"Scalar::Util": "Scalar-List-Utils", "List::Util": "Scalar-List-Utils", "List::Util::XS": "Scalar-List-Utils",
	"Sub::Util": "Scalar-List-Utils", "File::Spec": "PathTools", "File::Spec::": "PathTools", "Cwd": "PathTools",
	"Net::FTP": "libnet", "Net::SMTP": "libnet", "Net::POP3": "libnet", "Net::NNTP": "libnet", "Net::Cmd": "libnet",
	"Net::Domain": "libnet", "Net::Config": "libnet", "Net::Netrc": "libnet", "Net::Time": "libnet",
	"Pod::Man": "podlators", "Pod::Text": "podlators", "Pod::Text::": "podlators", "Pod::ParseLink": "podlators",
	"Pod::Parser": "Pod-Parser", "Pod::Select": "Pod-Parser", "Pod::PlainText": "Pod-Parser",
	"Pod::Simple::": "Pod-Simple", "Text::Wrap": "Text-Tabs+Wrap", "Text::Tabs": "Text-Tabs+Wrap",
	"Math::Trig": "Math-Complex", "Math::BigFloat": "Math-BigInt", "Math::BigInt::Calc": "Math-BigInt",
	"bigint": "bignum", "bigrat": "bignum", "bigfloat": "bignum", "Fatal": "autodie", "autodie::": "autodie",
	"IO::Handle": "IO", "IO::File": "IO", "IO::Socket": "IO", "IO::Socket::INET": "IO", "IO::Socket::UNIX": "IO",
	"IO::Select": "IO", "IO::Seekable": "IO", "IO::Dir": "IO", "IO::Pipe": "IO", "IO::Poll": "IO",
	"Compress::Zlib": "IO-Compress", "IO::Compress::": "IO-Compress", "IO::Uncompress::": "IO-Compress",
	"IO::Compress::Brotli": "IO-Compress-Brotli", "IO::Socket::SSL::": "IO-Socket-SSL",
	"Filter::Util::Call": "Filter", "Digest::base": "Digest", "Digest::file": "Digest",
	"Encode::": "Encode", "Encode::Locale": "Encode-Locale", "Time::Seconds": "Time-Piece",
	"threads::shared": "threads-shared", "version::": "version", "Exporter::Heavy": "Exporter", "Carp::Heavy": "Carp",
	"ExtUtils::MM": "ExtUtils-MakeMaker", "ExtUtils::MY": "ExtUtils-MakeMaker", "ExtUtils::MakeMaker::": "ExtUtils-MakeMaker",
	"JSON::PP::Boolean": "JSON-PP", "MIME::QuotedPrint": "MIME-Base64", "Getopt::Long::Parser": "Getopt-Long",
	// others commonly imported by a module other than the main one
	"YAML::XS": "YAML-LibYAML", "YAML::XS::": "YAML-LibYAML", "XML::LibXML::": "XML-LibXML",
	"XML::Parser::": "XML-Parser", "DBI::": "DBI", "DBIx::Class::": "DBIx-Class",
	"DBIx::Class::Schema::Loader": "DBIx-Class-Schema-Loader", "DBIx::Class::Schema::Loader::": "DBIx-Class-Schema-Loader",
	"Log::Log4perl::": "Log-Log4perl", "PPI::": "PPI", "PPIx::Regexp::": "PPIx-Regexp", "PPIx::Utils::": "PPIx-Utils",
	"IO::Async::": "IO-Async", "IO::Async::SSL": "IO-Async-SSL", "Future::Utils": "Future",
	"Net::SSLeay::": "Net-SSLeay", "Sub::Exporter::Util": "Sub-Exporter", "Email::MIME::": "Email-MIME",
	"Tk::": "Tk", "Gtk3::": "Gtk3", "Wx::": "Wx", "AnyEvent::Util": "AnyEvent", "AnyEvent::Handle": "AnyEvent",
	"AnyEvent::Socket": "AnyEvent", "AnyEvent::Loop": "AnyEvent", "AnyEvent::Strict": "AnyEvent",
}

// walkStop are namespace segments whose modules are separate distributions by
// convention (Catalyst::Plugin::X, Plack::Middleware::X, DateTime::Format::X): a
// module under one is not attributed to a distribution the namespace above it
// names.
var walkStop = map[string]bool{"Plugin": true, "Plugins": true, "Middleware": true, "Format": true, "Extension": true}

// distOf is the distribution providing a module, by distAliases or the module's own
// name.
//
// Implements: REQ-PERL-008
func distOf(module string) string {
	if d, ok := distAliases[module]; ok && d != "" {
		return d
	}
	segments := strings.Split(module, "::")
	for k := len(segments) - 1; k >= 1; k-- {
		if d, ok := distAliases[strings.Join(segments[:k], "::")+"::"]; ok {
			if d == "" {
				break
			}
			return d
		}
	}
	return strings.ReplaceAll(module, "::", "-")
}

// candidates are the distributions a module may belong to, most specific first: its
// own (distOf), then those of the namespaces above it (Plack::Request -> Plack),
// stopping at a plug-in namespace.
func candidates(module string) []string {
	out := []string{distOf(module)}
	segments := strings.Split(module, "::")
	for k := len(segments) - 1; k >= 1; k-- {
		if walkStop[segments[k-1]] {
			break
		}
		d := distOf(strings.Join(segments[:k], "::"))
		if d != out[len(out)-1] {
			out = append(out, d)
		}
	}
	return out
}
