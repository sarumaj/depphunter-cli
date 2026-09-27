package proto

import "strings"

// knownModules maps the import paths of widely used third-party protos to the Buf
// Schema Registry modules that publish them, most usual first. A path the project does
// not have is attributed to the module of the longest matching prefix; where a
// buf.yaml declares one of the candidates, that one.
//
// Implements: REQ-PROTO-006
var knownModules = []struct {
	prefix  string
	modules []string
}{
	{"google/", []string{"buf.build/googleapis/googleapis"}}, // google/api, rpc, type, longrunning, cloud...
	{"validate/", []string{"buf.build/envoyproxy/protoc-gen-validate"}},
	{"buf/validate/", []string{"buf.build/bufbuild/protovalidate"}},
	{"grpc/", []string{"buf.build/grpc/grpc"}},
	{"gogoproto/", []string{"buf.build/gogo/protobuf", "buf.build/cosmos/gogo-proto"}},
	{"github.com/gogo/protobuf/gogoproto/", []string{"buf.build/gogo/protobuf", "buf.build/cosmos/gogo-proto"}},
	{"protoc-gen-openapiv2/options/", []string{"buf.build/grpc-ecosystem/grpc-gateway"}},
	{"protoc-gen-openapiv3/options/", []string{"buf.build/grpc-ecosystem/grpc-gateway"}},
	{"envoy/", []string{"buf.build/envoyproxy/envoy"}},
	{"udpa/", []string{"buf.build/cncf/xds"}},
	{"xds/", []string{"buf.build/cncf/xds"}},
	{"opentelemetry/proto/", []string{"buf.build/opentelemetry/opentelemetry"}},
	{"io/prometheus/client/", []string{"buf.build/prometheus/client-model"}},
	{"cosmos_proto/", []string{"buf.build/cosmos/cosmos-proto"}},
	{"cosmos/", []string{"buf.build/cosmos/cosmos-sdk"}},
	{"amino/", []string{"buf.build/cosmos/cosmos-sdk"}},
}

// known returns the candidate modules for an import path, nil when the table has none.
func known(name string) []string {
	best := -1
	for i, k := range knownModules {
		if strings.HasPrefix(name, k.prefix) && (best < 0 || len(k.prefix) > len(knownModules[best].prefix)) {
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	return knownModules[best].modules
}

// wellKnown reports whether an import is one of the protos protoc and buf ship
// themselves: the well-known types (google/protobuf/any.proto, timestamp.proto...),
// descriptor.proto, the compiler plugin protocol and the edition feature files.
//
// Implements: REQ-PROTO-004
func wellKnown(name string) bool { return strings.HasPrefix(name, "google/protobuf/") }
