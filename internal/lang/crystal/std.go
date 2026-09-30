package crystal

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// stdlib holds the top-level names of Crystal's standard library (the files and
// directories of the compiler's src/, as of Crystal 1.x): `require "json"`,
// `require "http/client"`, `require "digest/sha256"` and
// `require "compiler/crystal/syntax"` name them. db, pg, markd and the like are
// shards, not the standard library.
//
// Implements: REQ-CRYSTAL-007
var stdlib = lang.WordSet(`annotations array atomic base64 benchmark big bit_array bool box channel char class
colorize comparable compiler complex compress concurrent crypto crystal csv deque digest dir ecr enum
enumerable env errno exception fiber file file_utils float gc hash html http humanize indexable ini int
intrinsics io iterable iterator json kernel levenshtein lib_c lib_z llvm log macros math mime mutex
named_tuple nil number oauth oauth2 object openssl option_parser path pointer prelude pretty_print
primitives proc process raise random range reference reference_storage regex semantic_version set
signal slice socket spec static_array steppable string string_pool string_scanner struct symbol sync
syscall system system_error termios time tuple unicode union uri uuid va_list value wait_group
wasi_error weak_ref winerror xml yaml`)

// fold makes shard names comparable: a require's first segment is the shard's
// name as a rule, but shards and requires spell it with - or _, and a shard.yml
// may key a dependency by its repository's name, often with a crystal- prefix or
// a .cr suffix (crystal-sqlite3 for sqlite3).
func fold(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "-", "_"))
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".cr"), "_cr")
	return strings.TrimPrefix(s, "crystal_")
}
