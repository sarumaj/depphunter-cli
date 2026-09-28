package perl

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// An application with a cpanfile and Carton's cpanfile.snapshot (lib/ modules,
// t/ tests with FindBin and relative use lib, a PSGI app, a script with a perl #!
// line and no extension, Moose roles and classes, a Corinna class), a CPAN
// distribution (dist/: Makefile.PL, META.json, dist.ini) and a Module::Build one
// (built/: Build.PL, META.yml) below it, and what is not claimed: Carton's local/,
// a Prolog .pl and a .t that is not Perl.
//
// Verifies: REQ-PERL-001, REQ-PERL-002, REQ-PERL-004, REQ-PERL-005, REQ-PERL-006
// Verifies: REQ-PERL-007, REQ-PERL-008
func TestImportsAndManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	imports := map[string]map[string]lang.Target{
		"app.psgi": {
			"use Plack::Builder": {Ecosystem: ecosystemCPAN, Package: "Plack", Version: "1.0050", Requested: ">= 1.0047", Pinned: true},
			"use Shop":           {Local: "lib/Shop.pm"},
		},
		"built/Build.PL": {
			"use Module::Build":         {Ecosystem: ecosystemCPAN, Package: "Module-Build", Unresolved: true},
			"requires Path::Tiny 0.100": {Ecosystem: ecosystemCPAN, Package: "Path-Tiny", Version: ">= 0.100", Floating: true},
			"build requires Test::More": {Ecosystem: ecosystemCPAN, Package: "Test-Simple", Floating: true},
		},
		"built/META.yml": {
			"requires Path::Tiny 0.100": {Ecosystem: ecosystemCPAN, Package: "Path-Tiny", Version: ">= 0.100", Floating: true},
			"build requires Test::More": {Ecosystem: ecosystemCPAN, Package: "Test-Simple", Floating: true},
		},
		"built/lib/Acme/Built.pm": {
			"use Path::Tiny": {Ecosystem: ecosystemCPAN, Package: "Path-Tiny", Version: ">= 0.100", Floating: true},
		},
		"cpanfile": {
			"requires Plack 1.0047":              {Ecosystem: ecosystemCPAN, Package: "Plack", Version: "1.0050", Requested: ">= 1.0047", Pinned: true},
			"requires Moose":                     {Ecosystem: ecosystemCPAN, Package: "Moose", Version: "2.2206", Pinned: true},
			"requires LWP::UserAgent == 6.72":    {Ecosystem: ecosystemCPAN, Package: "libwww-perl", Version: "6.72", Pinned: true},
			"requires Try::Tiny >= 0.30, < 1":    {Ecosystem: ecosystemCPAN, Package: "Try-Tiny", Version: "0.31", Requested: ">= 0.30, < 1", Pinned: true},
			"requires List::Util 1.45":           {Ecosystem: ecosystemCPAN, Package: "Scalar-List-Utils", Version: "1.63", Requested: ">= 1.45", Pinned: true},
			"requires Exporter":                  {Ecosystem: ecosystemStd, Package: "Exporter"},
			"requires JSON::MaybeXS == 1.004005": {Ecosystem: ecosystemCPAN, Package: "JSON-MaybeXS", Version: "1.004005", Pinned: true},
			"recommends JSON::XS":                {Ecosystem: ecosystemCPAN, Package: "JSON-XS", Floating: true},
			"requires Acme::Git":                 {Ecosystem: ecosystemCPAN, Package: "Acme-Git", Origin: "https://github.com/acme/p5-acme-git.git"},
			"test requires Test::More 0.98":      {Ecosystem: ecosystemCPAN, Package: "Test-Simple", Version: ">= 0.98", Floating: true},
			"test requires Test::Deep":           {Ecosystem: ecosystemCPAN, Package: "Test-Deep", Floating: true},
			"develop requires Perl::Critic":      {Ecosystem: ecosystemCPAN, Package: "Perl-Critic", Floating: true},
			"requires DBD::SQLite":               {Ecosystem: ecosystemCPAN, Package: "DBD-SQLite", Floating: true},
		},
		"dist/META.json": {
			"requires Moo":                    {Ecosystem: ecosystemCPAN, Package: "Moo", Floating: true},
			"test requires Test::Fatal 0.016": {Ecosystem: ecosystemCPAN, Package: "Test-Fatal", Version: ">= 0.016", Floating: true},
		},
		"dist/Makefile.PL": {
			"use ExtUtils::MakeMaker":         {Ecosystem: ecosystemStd, Package: "ExtUtils::MakeMaker"},
			"requires JSON::PP 4.00":          {Ecosystem: ecosystemCPAN, Package: "JSON-PP", Version: ">= 4.00", Floating: true},
			"requires Moo":                    {Ecosystem: ecosystemCPAN, Package: "Moo", Floating: true},
			"test requires Test::Fatal 0.016": {Ecosystem: ecosystemCPAN, Package: "Test-Fatal", Version: ">= 0.016", Floating: true},
			"recommends Cpanel::JSON::XS 4.0": {Ecosystem: ecosystemCPAN, Package: "Cpanel-JSON-XS", Version: ">= 4.0", Floating: true},
		},
		"dist/dist.ini": {
			"requires Moo 2.0":                {Ecosystem: ecosystemCPAN, Package: "Moo", Version: ">= 2.0", Floating: true},
			"test requires Test::Fatal 0.016": {Ecosystem: ecosystemCPAN, Package: "Test-Fatal", Version: ">= 0.016", Floating: true},
			"recommends Cpanel::JSON::XS 4":   {Ecosystem: ecosystemCPAN, Package: "Cpanel-JSON-XS", Version: ">= 4", Floating: true},
			"develop requires Test::Pod 1.41": {Ecosystem: ecosystemCPAN, Package: "Test-Pod", Version: ">= 1.41", Floating: true},
		},
		"dist/lib/Acme/Widget.pm": {
			"use Moo":                {Ecosystem: ecosystemCPAN, Package: "Moo", Floating: true},
			"use JSON::PP":           {Ecosystem: ecosystemCPAN, Package: "JSON-PP", Version: ">= 4.00", Floating: true},
			"use Acme::Widget::Util": {},
			"use Plack":              {Ecosystem: ecosystemCPAN, Package: "Plack", Version: "1.0050", Requested: ">= 1.0047", Pinned: true},
			"use Cpanel::JSON::XS":   {Ecosystem: ecosystemCPAN, Package: "Cpanel-JSON-XS", Version: ">= 4.0", Floating: true},
		},
		"examples/tool/bin/run": {
			"use Mojo::File": {Ecosystem: ecosystemCPAN, Package: "Mojolicious", Unresolved: true},
			"use Tool":       {Local: "examples/tool/modules/Tool.pm"},
		},
		"examples/tool/bin/run2": {
			"use File::Spec":     {Ecosystem: ecosystemStd, Package: "File::Spec"},
			"use File::Basename": {Ecosystem: ecosystemStd, Package: "File::Basename"},
			"use Tool":           {Local: "examples/tool/modules/Tool.pm"},
			"require \"$FindBin::Bin/../modules/Tool.pm\"": {Local: "examples/tool/modules/Tool.pm"},
		},
		"lib/Shop.pm": {
			"use Moose":                     {Ecosystem: ecosystemCPAN, Package: "Moose", Version: "2.2206", Pinned: true},
			"use Shop::Cart":                {Local: "lib/Shop/Cart.pm"},
			"use Plack::Request":            {Ecosystem: ecosystemCPAN, Package: "Plack", Version: "1.0050", Requested: ">= 1.0047", Pinned: true},
			"use LWP::UserAgent":            {Ecosystem: ecosystemCPAN, Package: "libwww-perl", Version: "6.72", Pinned: true},
			"use Scalar::Util":              {Ecosystem: ecosystemCPAN, Package: "Scalar-List-Utils", Version: "1.63", Requested: ">= 1.45", Pinned: true},
			"use Data::Dumper":              {Ecosystem: ecosystemStd, Package: "Data::Dumper"},
			"use POSIX":                     {Ecosystem: ecosystemStd, Package: "POSIX"},
			"use Mojo::UserAgent":           {Ecosystem: ecosystemCPAN, Package: "Mojolicious", Unresolved: true},
			"use DateTime::Format::ISO8601": {Ecosystem: ecosystemCPAN, Package: "DateTime-Format-ISO8601", Unresolved: true},
			"use Some::Unknown::Thing":      {Ecosystem: ecosystemCPAN, Package: "Some-Unknown-Thing", Unresolved: true},
			"use constant":                  {Ecosystem: ecosystemStd, Package: "constant"},
			"with Shop::Role::Priced":       {Local: "lib/Shop/Role/Priced.pm"},
			"extends Shop::Base":            {Local: "lib/Shop/Base.pm"},
			"require Try::Tiny":             {Ecosystem: ecosystemCPAN, Package: "Try-Tiny", Version: "0.31", Requested: ">= 0.30, < 1", Pinned: true},
			"use JSON::XS":                  {Ecosystem: ecosystemCPAN, Package: "JSON-XS", Floating: true},
		},
		"lib/Shop/Cart.pm": {
			"use parent":        {Ecosystem: ecosystemStd, Package: "parent"},
			"parent Shop::Base": {Local: "lib/Shop/Base.pm"},
			"use base":          {Ecosystem: ecosystemStd, Package: "base"},
			"base Exporter":     {Ecosystem: ecosystemStd, Package: "Exporter"},
			"use Try::Tiny":     {Ecosystem: ecosystemCPAN, Package: "Try-Tiny", Version: "0.31", Requested: ">= 0.30, < 1", Pinned: true},
		},
		"lib/Shop/Point.pm": {
			"use experimental": {Ecosystem: ecosystemStd, Package: "experimental"},
			":isa(Shop::Base)": {Local: "lib/Shop/Base.pm"},
		},
		"lib/Shop/Role/Priced.pm": {
			"use Moose::Role": {Ecosystem: ecosystemCPAN, Package: "Moose", Version: "2.2206", Pinned: true},
		},
		"script/shop": {
			"use Mojo::File": {Ecosystem: ecosystemCPAN, Package: "Mojolicious", Unresolved: true},
			"use Shop":       {Local: "lib/Shop.pm"},
			"use Shop::Base": {Local: "lib/Shop/Base.pm"},
		},
		"t/basic.t": {
			"use Test::More":                       {Ecosystem: ecosystemCPAN, Package: "Test-Simple", Version: ">= 0.98", Floating: true},
			"use Test::Deep":                       {Ecosystem: ecosystemCPAN, Package: "Test-Deep", Floating: true},
			"use FindBin":                          {Ecosystem: ecosystemStd, Package: "FindBin"},
			"use Shop::Inc":                        {Local: "inc/Shop/Inc.pm"},
			"use TestHelper":                       {Local: "t/testlib/TestHelper.pm"},
			"use Shop":                             {Local: "lib/Shop.pm"},
			"use JSON::MaybeXS":                    {Ecosystem: ecosystemCPAN, Package: "JSON-MaybeXS", Version: "1.004005", Pinned: true},
			"do \"$FindBin::Bin/data/fixture.pl\"": {Local: "t/data/fixture.pl"},
			"require \"t/data/fixture.pl\"":        {Local: "t/data/fixture.pl"},
			"require \"missing.pl\"":               {},
			"use_ok Shop::Cart":                    {Local: "lib/Shop/Cart.pm"},
		},
		"t/testlib/TestHelper.pm": {
			"use Exporter": {Ecosystem: ecosystemStd, Package: "Exporter"},
		},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, results[file], want) })
	}
	for _, f := range []string{"local/lib/perl5/Plack.pm", "prolog/family.pl", "templates/page.t", "cpanfile.snapshot"} {
		if _, ok := results[f]; ok {
			t.Errorf("%s: claimed", f)
		}
	}
}

// Packages (statement and block form, Corinna classes), subs (methods when they
// take $self or $class, forward declarations skipped), use constant's names and
// the attributes Moose's has declares; nothing from heredocs, POD, strings or
// after __END__.
//
// Verifies: REQ-PERL-003
func TestDefinitions(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	symbols := map[string]map[string]string{
		"app.psgi":                      {},
		"built/Build.PL":                {},
		"built/META.yml":                {},
		"built/lib/Acme/Built.pm":       {"Acme::Built": "class"},
		"cpanfile":                      {},
		"dist/META.json":                {},
		"dist/Makefile.PL":              {},
		"dist/dist.ini":                 {},
		"dist/lib/Acme/Widget.pm":       {"Acme::Widget": "class"},
		"examples/tool/bin/run":         {},
		"examples/tool/bin/run2":        {},
		"examples/tool/modules/Tool.pm": {"Tool": "class", "Tool.go": "function"},
		"inc/Shop/Inc.pm":               {"Shop::Inc": "class"},
		"lib/Shop.pm":                   {"Shop": "class", "Shop.TAX": "const", "Shop.CURRENCY": "const", "Shop.LIMIT": "const", "Shop.name": "property", "Shop.price": "property", "Shop.qty": "property", "Shop.id": "property", "Shop.total": "method", "Shop._helper": "function"},
		"lib/Shop/Base.pm":              {"Shop::Base": "class", "Shop::Base.new": "function"},
		"lib/Shop/Cart.pm":              {"Shop::Cart": "class", "Shop::Cart.new": "method", "Shop::Cart.add": "method", "Shop::Cart::Item": "class", "Shop::Cart::Item.price": "function", "Shop::Cart.count": "function"},
		"lib/Shop/Point.pm":             {"Shop::Point": "class", "Shop::Point.coords": "method"},
		"lib/Shop/Role/Priced.pm":       {"Shop::Role::Priced": "class"},
		"other/Shop/Inc.pm":             {"Shop::Inc": "class"},
		"other/TestHelper.pm":           {"TestHelper": "class"},
		"other/Tool.pm":                 {"Tool": "class"},
		"script/shop":                   {"run": "function"},
		"t/basic.t":                     {},
		"t/data/fixture.pl":             {},
		"t/testlib/TestHelper.pm":       {"TestHelper": "class", "TestHelper.helper": "function"},
	}
	for file, want := range symbols {
		t.Run(file, func(t *testing.T) { langtest.CheckSymbols(t, results[file], want) })
	}
}

// cpanfile.snapshot records what each installed distribution requires: required
// modules become the distributions the snapshot says provide them (pinned), a core
// module nothing installed is left out, and any other is named after the module.
//
// Verifies: REQ-PERL-007
func TestSnapshotDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"), Plugin{})
	got := r.Dependencies(lang.Target{Ecosystem: ecosystemCPAN, Package: "Plack", Version: "1.0050", Pinned: true})
	want := []lang.Target{
		{Ecosystem: ecosystemCPAN, Package: "HTTP-Message", Version: "6.45", Pinned: true},
		{Ecosystem: ecosystemCPAN, Package: "Try-Tiny", Version: "0.31", Pinned: true},
		{Ecosystem: ecosystemCPAN, Package: "Hash-MultiValue", Version: ">= 0.05", Floating: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Plack: got %+v, want %+v", got, want)
	}
	got = r.Dependencies(lang.Target{Ecosystem: ecosystemCPAN, Package: "Moose", Version: "2.2206", Pinned: true})
	want = []lang.Target{
		{Ecosystem: ecosystemCPAN, Package: "Scalar-List-Utils", Version: "1.63", Pinned: true},
		{Ecosystem: ecosystemCPAN, Package: "Try-Tiny", Version: "0.31", Pinned: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Moose: got %+v, want %+v", got, want)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemCPAN, Package: "Plack", Version: "1.0", Pinned: true}); got != nil {
		t.Errorf("another version: %+v", got)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemStd, Package: "POSIX"}); got != nil {
		t.Errorf("core module: %+v", got)
	}
}

// The lexer's traps: what reads as a pattern or a division, a string, a comment or
// a variable depends on what came before, and POD, here-documents, formats and
// __END__ hide text that looks like code. Each source must give exactly the
// imports listed.
//
// Verifies: REQ-PERL-002, REQ-PERL-010
func TestScanner(t *testing.T) {
	for _, c := range []struct {
		name, source string
		want         []string
	}{
		{"division then pattern", "my $x = $a / 2 / 3; my @p = split /,/, $s;\nuse A;\n", []string{"A"}},
		{"pattern with quote", "if ($s =~ /it's/) { }\nuse A;\n", []string{"A"}},
		{"punctuation variables", "local $\" = ','; local $/; my $n = $#list; print \"$#{$r}\";\nuse A;\n", []string{"A"}},
		{"deref and hashes", "my %h = %$r; my $v = $h{s} + $h{y} + $h{q}; my %o = (s => 1, m => 2);\nuse A;\n", []string{"A"}},
		{"file test", "if (-s $file) { }\nuse A;\n", []string{"A"}},
		{"substitution with brackets", "$s =~ s{use B}\n  {use C}gex;\n$s =~ s(a)[b];\nuse A;\n", []string{"A"}},
		{"transliteration", "$s =~ tr/a-z/A-Z/; $s =~ y#a#b#;\nuse A;\n", []string{"A"}},
		{"quote operators", "my $q = q<use B <nested>>; my $qq = qq{use C}; my @w = qw(use D); my $re = m!use E!;\nuse A;\n", []string{"A"}},
		{"heredocs", "print <<~EOT, <<'RAW', <<\"QQ\";\n    use B;\n    EOT\nuse C;\nRAW\nuse D;\nQQ\nuse A;\n", []string{"A"}},
		{"heredoc and shift", "my $x = 1 << 2;\nuse A;\n", []string{"A"}},
		{"pod", "=head1 NAME\n\nuse B;\n\n=cut\n\nuse A;\n=pod\n\nuse C;\n", []string{"A"}},
		{"end", "use A;\n__END__\nuse B;\n", []string{"A"}},
		{"data", "use A;\n__DATA__\nuse B;\n", []string{"A"}},
		{"format", "format STDOUT =\n@<<< use B;\n$x\n.\nuse A;\n", []string{"A"}},
		{"prototypes", "sub max($;$) { }\nsub min :prototype($$) { }\nsub sig ($x, $) { }\nuse A;\n", []string{"A"}},
		{"readline", "while (<$fh>) { } my @l = <STDIN>; while (<<>>) { }\nuse A;\n", []string{"A"}},
		{"comment with quote", "my $x = 1; # don't\nuse A;\n", []string{"A"}},
		{"string eval", "eval \"use B; 1\" or warn;\neval q{require C};\nuse A;\n", []string{"B", "C", "A"}},
		{"use if and aliased", "use if $] < 5.010, 'MRO::Compat';\nuse aliased 'My::Long::Name' => 'Short';\n", []string{"MRO::Compat", "aliased", "My::Long::Name"}},
		{"require forms", "require Foo::Bar; require 5.006; require v5.10; require $x; require Baz->import;\n", []string{"Foo::Bar"}},
		{"moose only with moose", "with 'NotARole';\nuse Moo;\nwith 'Role::A', 'Role::B' => { -excludes => 'x' };\nextends qw(Base);\n", []string{"Moo", "Role::A", "Role::B", "Base"}},
		{"mojo base", "use Mojo::Base 'Mojolicious::Controller', -signatures;\nhas 'x';\n", []string{"Mojo::Base", "Mojolicious::Controller"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, rawImport := range readSource([]byte(c.source)).Imports {
				got = append(got, rawImport.Module)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Paths computed from where a file is: FindBin, __FILE__, dirname, File::Spec,
// Mojo::File and Path::Tiny chains; anything else is unknown.
//
// Verifies: REQ-PERL-004, REQ-PERL-009
func TestEvalPaths(t *testing.T) {
	for source, want := range map[string]string{
		`"$FindBin::Bin/../lib"`:                           "\x01/../../lib",
		`$FindBin::RealBin . '/lib'`:                       "\x01/../lib",
		`File::Spec->catdir(dirname(__FILE__), 'lib')`:     "\x01/../lib",
		`curfile->dirname->sibling('lib')->to_string`:      "\x01/../../lib",
		`path(__FILE__)->parent->child('lib', 'perl5')`:    "\x01/../lib/perl5",
		`Cwd::abs_path(File::Basename::dirname(__FILE__))`: "\x01/..",
		`'t/lib'`:          "t/lib",
		`"$ENV{HOME}/lib"`: "",
		`$dir`:             "",
		`somefunc('lib')`:  "",
	} {
		got, _ := evalPath(lex([]byte(source)))
		if got != want {
			t.Errorf("%s: got %q, want %q", source, got, want)
		}
	}
}

// The manifests and the snapshot keep their files' distinction by class, so a
// cached extraction of a cpanfile is not read back for a source.
//
// Verifies: REQ-PERL-001
func TestClass(t *testing.T) {
	for p, want := range map[string]string{
		"cpanfile": classCpanfile, "dist/Makefile.PL": classMakefile, "Build.PL": classBuild,
		"META.json": classMeta, "MYMETA.yml": classMeta, "dist.ini": classDistIni, "lib/Foo.pm": "", "gen/Foo.pm.PL": "",
	} {
		if got := (Plugin{}).Class(&scan.File{Path: p}); got != want {
			t.Errorf("%s: %q, want %q", p, got, want)
		}
	}
}

// Every prefix of every fixture file, and inputs cut inside each construct,
// extract without a panic; deep nesting does not exhaust the stack.
//
// Verifies: REQ-PERL-010
func TestTruncated(t *testing.T) {
	var sources [][]byte
	filepath.Walk("testdata/repo", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			b, _ := os.ReadFile(p)
			sources = append(sources, b)
		}
		return nil
	})
	for _, s := range []string{"<<\"", "<<~", "s{a}", "s{a}{", "tr/a/", "q(", "qw[a", "$#", "${", "@{", "%$",
		"=pod\n", "format X =\n", "sub f($", "use lib curfile->", "use constant {", "has [", "class X :isa(", "\"\\"} {
		sources = append(sources, []byte("use A;\n"+s))
	}
	for _, source := range sources {
		for i := 0; i <= len(source); i++ {
			readSource(source[:i])
			for _, class := range []string{classCpanfile, classMakefile, classDistIni} {
				readManifest(class, "x", source[:i])
			}
		}
		readManifest(classMeta, "META.json", source)
		readManifest(classMeta, "META.yml", source)
		readSnapshot(source)
	}
	deep := strings.Repeat("{[(", 100000) + "use A;"
	readSource([]byte(deep))
	readManifest(classMakefile, "Makefile.PL", []byte("WriteMakefile(PREREQ_PM => "+deep))
}
