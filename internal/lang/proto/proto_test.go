package proto

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Three kinds of project in one tree: a buf v1 workspace (buf.work.yaml listing proto
// and third_party, a v1 buf.yaml whose deps are a floating module, a label pinned by
// the v1 buf.lock and an unlabeled module the lock pins, a v1 buf.gen.yaml); a buf v2
// workspace (modules importing each other, a commit reference, a label the v2 lock pins, a
// module only the lock records, a v2 buf.gen.yaml with inputs); and protoc-style
// protos with no Buf configuration under src/main/proto, which find their imports
// under the conventional root, in the importer's directory and by a unique suffix,
// and name what they cannot find. Well-known types, import public and import weak,
// editions and proto2 groups, nested messages, oneofs, services with rpc options,
// extend blocks and option aggregates are in the sources.
//
// Verifies: REQ-PROTO-001, REQ-PROTO-002, REQ-PROTO-003, REQ-PROTO-004, REQ-PROTO-005
// Verifies: REQ-PROTO-006, REQ-PROTO-007, REQ-PROTO-008
func TestImportsAndBufModules(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	local := func(p string) lang.Target { return lang.Target{Local: p} }
	std := func(p string) lang.Target { return lang.Target{Ecosystem: ecosystemStd, Package: p} }
	googleV1 := lang.Target{Ecosystem: ecoBuf, Package: "buf.build/googleapis/googleapis", Version: "62f35d8aed1149c291d606d958a7ce32", Pinned: true}
	googleV2 := lang.Target{Ecosystem: ecoBuf, Package: "buf.build/googleapis/googleapis", Version: "e7f8d366f5264595bcc4cd4139af9973", Pinned: true}
	pgv := lang.Target{Ecosystem: ecoBuf, Package: "buf.build/envoyproxy/protoc-gen-validate", Version: "6607b10f00ed4a3d98f906807131c44a",
		Requested: "v1.0.4", Pinned: true}
	payments := lang.Target{Ecosystem: ecoBuf, Package: "buf.build/acme/payments", Floating: true}
	protovalidate := lang.Target{Ecosystem: ecoBuf, Package: "buf.build/bufbuild/protovalidate", Version: "0123456789abcdef0123456789abcdef", Pinned: true}
	gateway := lang.Target{Ecosystem: ecoBuf, Package: "buf.build/grpc-ecosystem/grpc-gateway", Version: "4c5ba75caaf84e928b7137ae5c18c26a",
		Requested: "v2.19.0", Pinned: true}
	types := local("v1/third_party/common/v1/types.proto")
	imports := map[string]map[string]lang.Target{
		"v1/buf.work.yaml": {"proto/": local("v1/proto"), "third_party/": local("v1/third_party")},
		"v1/buf.gen.yaml": {
			"buf.build/protocolbuffers/go:v1.31.0": {Ecosystem: ecoBuf, Package: "buf.build/protocolbuffers/go", Version: "v1.31.0", Pinned: true, Registry: remotePlugin},
			"buf.build/grpc/go":                    {Ecosystem: ecoBuf, Package: "buf.build/grpc/go", Floating: true, Registry: remotePlugin},
		},
		"v1/proto/buf.yaml": {
			"buf.build/googleapis/googleapis":                 googleV1,
			"buf.build/envoyproxy/protoc-gen-validate:v1.0.4": pgv,
			"buf.build/acme/payments":                         payments,
		},
		"v1/proto/buf.lock": {
			"buf.build/googleapis/googleapis":          googleV1,
			"buf.build/envoyproxy/protoc-gen-validate": {Ecosystem: ecoBuf, Package: "buf.build/envoyproxy/protoc-gen-validate", Version: "6607b10f00ed4a3d98f906807131c44a", Pinned: true},
		},
		"v1/proto/acme/billing/v1/invoice.proto": {
			`import "google/protobuf/timestamp.proto"`:   std("google/protobuf/timestamp.proto"),
			`import "google/api/annotations.proto"`:      googleV1,
			`import "validate/validate.proto"`:           pgv,
			`import "acme/billing/v1/money.proto"`:       local("v1/proto/acme/billing/v1/money.proto"),
			`import "common/v1/types.proto"`:             types,
			`import "payments/v1/pay.proto"`:             payments,
			`import weak "acme/billing/v1/legacy.proto"`: local("v1/proto/acme/billing/v1/legacy.proto"),
		},
		"v1/proto/acme/billing/v1/money.proto":  {`import public "common/v1/types.proto"`: types},
		"v1/proto/acme/billing/v1/legacy.proto": {},
		"v1/third_party/common/v1/types.proto":  {`import "google/protobuf/descriptor.proto"`: std("google/protobuf/descriptor.proto")},
		"v2/buf.yaml": {
			"api/":            local("v2/api"),
			"internal/proto/": local("v2/internal/proto"),
			"buf.build/bufbuild/protovalidate:0123456789abcdef0123456789abcdef": protovalidate,
			"buf.build/grpc-ecosystem/grpc-gateway:v2.19.0":                     gateway,
		},
		"v2/buf.lock": {
			"buf.build/googleapis/googleapis":       googleV2,
			"buf.build/grpc-ecosystem/grpc-gateway": {Ecosystem: ecoBuf, Package: "buf.build/grpc-ecosystem/grpc-gateway", Version: "4c5ba75caaf84e928b7137ae5c18c26a", Pinned: true},
		},
		"v2/buf.gen.yaml": {
			"buf.build/protocolbuffers/go:v1.34.1": {Ecosystem: ecoBuf, Package: "buf.build/protocolbuffers/go", Version: "v1.34.1", Pinned: true, Registry: remotePlugin},
			"api/":                                 local("v2/api"),
			"buf.build/acme/other:main":            {Ecosystem: ecoBuf, Package: "buf.build/acme/other", Version: "main"},
		},
		"v2/api/shop/v1/shop.proto": {
			`import "buf/validate/validate.proto"`:                    protovalidate,
			`import "protoc-gen-openapiv2/options/annotations.proto"`: gateway,
			`import "google/type/money.proto"`:                        googleV2,
			`import "audit/v1/audit.proto"`:                           local("v2/internal/proto/audit/v1/audit.proto"),
			`import "payments/v1/pay.proto"`:                          {Ecosystem: ecoBuf, Package: "payments", Unresolved: true},
		},
		"v2/internal/proto/audit/v1/audit.proto": {`import "google/protobuf/descriptor.proto"`: std("google/protobuf/descriptor.proto")},
		"src/main/proto/com/acme/orders.proto": {
			`import "com/acme/items.proto"`:      local("src/main/proto/com/acme/items.proto"),
			`import "google/protobuf/any.proto"`: std("google/protobuf/any.proto"),
			`import "gogoproto/gogo.proto"`:      {Ecosystem: ecoBuf, Package: "buf.build/gogo/protobuf", Unresolved: true},
			`import "common/v1/types.proto"`:     types,
			`import "mystery/v1/thing.proto"`:    {Ecosystem: ecoBuf, Package: "mystery", Unresolved: true},
		},
		"src/main/proto/com/acme/items.proto":  {`import "shared.proto"`: local("src/main/proto/com/acme/shared.proto")},
		"src/main/proto/com/acme/shared.proto": {},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, results[file], want) })
	}

	symbols := map[string]map[string]string{
		"v1/proto/acme/billing/v1/invoice.proto": {
			"acme.billing.v1": "package", "Invoice": "message", "Invoice.Line": "message", "Invoice.Line.Kind": "enum",
			"Invoice.payer": "oneof", "Status": "enum", "InvoiceService": "service", "InvoiceService.GetInvoice": "rpc",
			"InvoiceService.ListInvoices": "rpc", "GetInvoiceRequest": "message",
		},
		"v1/third_party/common/v1/types.proto": {
			"common.v1": "package", "google.protobuf.FileOptions": "extend", "Meta": "message",
		},
		"v2/api/shop/v1/shop.proto": {"acme.shop.v1": "package", "Order": "message"},
		"v2/internal/proto/audit/v1/audit.proto": {
			"audit.v1": "package", "Entry": "message", "Entry.Actor": "message", "Entry.google.protobuf.MessageOptions": "extend",
		},
		"v2/buf.yaml": {},
	}
	for file, want := range symbols {
		t.Run("symbols "+file, func(t *testing.T) { langtest.CheckSymbols(t, results[file], want) })
	}
	lines := map[string]int{}
	for _, s := range results["v1/proto/acme/billing/v1/invoice.proto"].Symbols {
		lines[s.Name] = s.Line
	}
	if lines["Invoice"] != 19 || lines["InvoiceService.ListInvoices"] != 48 {
		t.Errorf("lines: %v", lines)
	}
}

// The scanner skips comments and strings that look like declarations, reads
// adjacent strings as one, keeps going past an option's aggregate value and an rpc's
// option block, does not take a field named message or group for a declaration, and
// survives an unterminated block.
//
// Verifies: REQ-PROTO-002, REQ-PROTO-003, REQ-PROTO-009
func TestScanner(t *testing.T) {
	source := []byte("\xef\xbb\xbf// import \"no.proto\";\n/* message No {} */\n" +
		"syntax = 'proto2';\nimport \"a/\" \"b.proto\";\nimport 'c\\'d.proto';\n" +
		"option (x) = { a: \"}\" b { c: 1 } };\n" +
		"message M { optional string message = 1; optional int32 group = 2 [(y) = { z: 1 }];\n" +
		"  enum E { A = 0; }\n  optional group G = 3 { optional int32 n = 1; }\n}\n" +
		"service S { rpc R (M) returns (M) { option (h) = { get: \"/x\" }; } rpc T (M) returns (M); }\n" +
		"message Open {\n  message Inner {\n")
	extraction := readProto(source)
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if want := []string{`import "a/b.proto"`, `import "c'd.proto"`}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %v, want %v", specs, want)
	}
	var names []string
	for _, s := range extraction.Symbols {
		names = append(names, s.Name)
	}
	if want := []string{"M", "M.E", "M.G", "S", "S.R", "S.T", "Open", "Open.Inner"}; !reflect.DeepEqual(names, want) {
		t.Errorf("symbols %v, want %v", names, want)
	}
}

// Verifies: REQ-PROTO-008
func TestModuleReferences(t *testing.T) {
	for in, want := range map[string][2]string{
		"buf.build/acme/pay":         {"buf.build/acme/pay", ""},
		"Buf.Build/Acme/Pay:v1.2.0":  {"buf.build/acme/pay", "v1.2.0"},
		"buf.corp.test:8443/a/b:c1":  {"buf.corp.test:8443/a/b", "c1"},
		"buf.corp.test:8443/a/b":     {"buf.corp.test:8443/a/b", ""},
		" buf.build/acme/pay:main  ": {"buf.build/acme/pay", "main"},
	} {
		if name, reference := moduleReference(in); name != want[0] || reference != want[1] {
			t.Errorf("%q: got %q %q, want %q", in, name, reference, want)
		}
	}
	for reference, want := range map[string]bool{
		"0123456789abcdef0123456789abcdef":         true,
		"01234567-89ab-cdef-0123-456789abcdef":     true,
		"0123456789abcdef0123456789abcdef01234567": true,
		"v1.2.0": false, "main": false, "0123456789ABCDEF0123456789ABCDEF": false,
	} {
		if got := commit(reference); got != want {
			t.Errorf("commit(%q) = %v", reference, got)
		}
	}
}

// Verifies: REQ-PROTO-001
func TestClaimsAndClasses(t *testing.T) {
	for p, want := range map[string]string{
		"api/v1/shop.proto": classProto, "x/Legacy.PROTO": classProto, "buf.yaml": classYAML, "proto/buf.yaml": classYAML,
		"buf.work.yaml": classWork, "buf.lock": classLock, "buf.gen.yaml": classGen, "buf.gen.go.yaml": classGen,
		"buf.yml": "", "config.yaml": "", "Cargo.lock": "", "main.go": "",
	} {
		if got := fileClass(p); got != want {
			t.Errorf("%s: class %q, want %q", p, got, want)
		}
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != (want != "") {
			t.Errorf("%s: claimed %v", p, got)
		}
	}
	key := func(p string) string {
		return cache.Key(Plugin{}.Name(), Plugin{}.Version(), lang.ClassOf(Plugin{}, &scan.File{Path: p}), []byte("version: v1"))
	}
	if key("buf.yaml") == key("buf.gen.yaml") || key("buf.yaml") == key("buf.work.yaml") {
		t.Error("files read differently share a cache key")
	}
}

// A Buf root above the repository is no root, a bare ".." no more than "../x":
// the module falls back to its own directory, so its import finds the module's
// file rather than a same-named one at the repository root.
//
// Verifies: REQ-LANG-031
func TestBufRootAboveTheRepository(t *testing.T) {
	for _, root := range []string{"../..", "../../x"} {
		t.Run(root, func(t *testing.T) {
			results := langtest.Analyze(t, Plugin{}, langtest.Write(t, map[string]string{
				"sub/buf.yaml": "version: v1beta1\nbuild:\n  roots:\n    - " + root + "\n",
				"sub/b.proto":  "syntax = \"proto3\";\nimport \"a.proto\";\n",
				"sub/a.proto":  "syntax = \"proto3\";\n",
				"a.proto":      "syntax = \"proto3\";\n",
			}))
			langtest.CheckImports(t, results["sub/b.proto"], map[string]lang.Target{`import "a.proto"`: {Local: "sub/a.proto"}})
		})
	}
}

// A directory a Buf file names above the repository is none of its directories,
// a bare ".." no more than "../x", even with the layout listing both: the check
// does not rely on the layout never holding a path above the root.
//
// Verifies: REQ-LANG-031
func TestBufDirectoryAboveTheRepository(t *testing.T) {
	r := newResolver(langtest.Files(t, langtest.Write(t, map[string]string{"buf.work.yaml": "version: v1\n"})))
	r.Directories[".."], r.Directories["../x"] = true, true
	for _, module := range []string{"..", "../x"} {
		if got := r.Resolve("buf.work.yaml", lang.RawImport{Module: module, Name: kindDirectory}); got != (lang.Target{}) {
			t.Errorf("%s: got %+v, want nothing", module, got)
		}
	}
}
