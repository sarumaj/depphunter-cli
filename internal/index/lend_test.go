package index

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/auth"
)

// A credential the repository binds to a feed is lent only past one gate: a store to
// lend to, a secret to lend, a feed that parses and names a host, and a host this
// machine vouches for. Each lending path (npm, NuGet, Python) is walked through every
// way past and short of it.
//
// Verifies: REQ-AUTH-023
func TestLendGate(t *testing.T) {
	lenders := map[string]func(c *Config, feed, secret string){
		"npm":    func(c *Config, feed, secret string) { c.lendNpm(feed, "", true, secret) },
		"nuget":  func(c *Config, feed, secret string) { c.lend(feed, "ci", secret) },
		"python": func(c *Config, feed, secret string) { c.lendPython(feed, "ci", secret) },
	}
	feeds := []string{
		"https://feed.corp/r",      // vouched for by host
		" https://feed.corp/r ",    // the same, padded
		"https://feed.corp/${X}",   // a variable the repository did not resolve
		"https://other.corp/r",     // vouched for by nobody
		"://feed.corp/r",           // does not parse
		"feed.corp/r",              // names no host
		"https://user@feed.corp/r", // carries a user name
	}
	var got []string
	for _, name := range []string{"npm", "nuget", "python"} {
		for _, feed := range feeds {
			for _, secret := range []string{"s3cret", ""} {
				for _, store := range []bool{true, false} {
					c := Discover(nil, environment(nil), "")
					var s *auth.Store
					if store {
						s = auth.Read(t.TempDir(), environment(nil))
						c.Credentials(s)
					}
					c.Trust([]string{"feed.corp"})
					lenders[name](c, feed, secret)
					sent := ""
					if s != nil {
						sent = authOf(s, "https://feed.corp/r/x")
					}
					got = append(got, fmt.Sprintf("%s %q %q store=%v: %q", name, feed, secret, store, sent))
				}
			}
		}
	}
	if want := strings.TrimSpace(lendGateWant); strings.Join(got, "\n") != want {
		t.Errorf("lend gate:\n%s", strings.Join(got, "\n"))
	}
}

const lendGateWant = `
npm "https://feed.corp/r" "s3cret" store=true: "Bearer s3cret"
npm "https://feed.corp/r" "s3cret" store=false: ""
npm "https://feed.corp/r" "" store=true: ""
npm "https://feed.corp/r" "" store=false: ""
npm " https://feed.corp/r " "s3cret" store=true: "Bearer s3cret"
npm " https://feed.corp/r " "s3cret" store=false: ""
npm " https://feed.corp/r " "" store=true: ""
npm " https://feed.corp/r " "" store=false: ""
npm "https://feed.corp/${X}" "s3cret" store=true: ""
npm "https://feed.corp/${X}" "s3cret" store=false: ""
npm "https://feed.corp/${X}" "" store=true: ""
npm "https://feed.corp/${X}" "" store=false: ""
npm "https://other.corp/r" "s3cret" store=true: ""
npm "https://other.corp/r" "s3cret" store=false: ""
npm "https://other.corp/r" "" store=true: ""
npm "https://other.corp/r" "" store=false: ""
npm "://feed.corp/r" "s3cret" store=true: ""
npm "://feed.corp/r" "s3cret" store=false: ""
npm "://feed.corp/r" "" store=true: ""
npm "://feed.corp/r" "" store=false: ""
npm "feed.corp/r" "s3cret" store=true: ""
npm "feed.corp/r" "s3cret" store=false: ""
npm "feed.corp/r" "" store=true: ""
npm "feed.corp/r" "" store=false: ""
npm "https://user@feed.corp/r" "s3cret" store=true: "Bearer s3cret"
npm "https://user@feed.corp/r" "s3cret" store=false: ""
npm "https://user@feed.corp/r" "" store=true: ""
npm "https://user@feed.corp/r" "" store=false: ""
nuget "https://feed.corp/r" "s3cret" store=true: "Basic Y2k6czNjcmV0"
nuget "https://feed.corp/r" "s3cret" store=false: ""
nuget "https://feed.corp/r" "" store=true: ""
nuget "https://feed.corp/r" "" store=false: ""
nuget " https://feed.corp/r " "s3cret" store=true: "Basic Y2k6czNjcmV0"
nuget " https://feed.corp/r " "s3cret" store=false: ""
nuget " https://feed.corp/r " "" store=true: ""
nuget " https://feed.corp/r " "" store=false: ""
nuget "https://feed.corp/${X}" "s3cret" store=true: "Basic Y2k6czNjcmV0"
nuget "https://feed.corp/${X}" "s3cret" store=false: ""
nuget "https://feed.corp/${X}" "" store=true: ""
nuget "https://feed.corp/${X}" "" store=false: ""
nuget "https://other.corp/r" "s3cret" store=true: ""
nuget "https://other.corp/r" "s3cret" store=false: ""
nuget "https://other.corp/r" "" store=true: ""
nuget "https://other.corp/r" "" store=false: ""
nuget "://feed.corp/r" "s3cret" store=true: ""
nuget "://feed.corp/r" "s3cret" store=false: ""
nuget "://feed.corp/r" "" store=true: ""
nuget "://feed.corp/r" "" store=false: ""
nuget "feed.corp/r" "s3cret" store=true: ""
nuget "feed.corp/r" "s3cret" store=false: ""
nuget "feed.corp/r" "" store=true: ""
nuget "feed.corp/r" "" store=false: ""
nuget "https://user@feed.corp/r" "s3cret" store=true: "Basic Y2k6czNjcmV0"
nuget "https://user@feed.corp/r" "s3cret" store=false: ""
nuget "https://user@feed.corp/r" "" store=true: ""
nuget "https://user@feed.corp/r" "" store=false: ""
python "https://feed.corp/r" "s3cret" store=true: "Basic Y2k6czNjcmV0"
python "https://feed.corp/r" "s3cret" store=false: ""
python "https://feed.corp/r" "" store=true: "Basic Y2k6"
python "https://feed.corp/r" "" store=false: ""
python " https://feed.corp/r " "s3cret" store=true: "Basic Y2k6czNjcmV0"
python " https://feed.corp/r " "s3cret" store=false: ""
python " https://feed.corp/r " "" store=true: "Basic Y2k6"
python " https://feed.corp/r " "" store=false: ""
python "https://feed.corp/${X}" "s3cret" store=true: ""
python "https://feed.corp/${X}" "s3cret" store=false: ""
python "https://feed.corp/${X}" "" store=true: ""
python "https://feed.corp/${X}" "" store=false: ""
python "https://other.corp/r" "s3cret" store=true: ""
python "https://other.corp/r" "s3cret" store=false: ""
python "https://other.corp/r" "" store=true: ""
python "https://other.corp/r" "" store=false: ""
python "://feed.corp/r" "s3cret" store=true: ""
python "://feed.corp/r" "s3cret" store=false: ""
python "://feed.corp/r" "" store=true: ""
python "://feed.corp/r" "" store=false: ""
python "feed.corp/r" "s3cret" store=true: ""
python "feed.corp/r" "s3cret" store=false: ""
python "feed.corp/r" "" store=true: ""
python "feed.corp/r" "" store=false: ""
python "https://user@feed.corp/r" "s3cret" store=true: "Basic Y2k6czNjcmV0"
python "https://user@feed.corp/r" "s3cret" store=false: ""
python "https://user@feed.corp/r" "" store=true: "Basic Y2k6"
python "https://user@feed.corp/r" "" store=false: ""
`

// A registry a package names by host is known when the user vouches for it by URL or
// by host, or this machine holds a credential for the host (each ecosystem's own
// test covers the credential).
//
// Verifies: REQ-SUP-016
func TestRegistryHostKnown(t *testing.T) {
	packages := map[string]string{
		TerraformModule: "tf.corp.test/acme/vpc/aws",
		Buf:             "BUF.Corp.test/acme/payments",
		OCI:             "oci.corp.test/acme/app:1",
	}
	var got []string
	for _, ecosystem := range []string{TerraformModule, Buf, OCI} {
		for _, trust := range [][]string{nil, {"https://tf.corp.test", "https://buf.corp.test", "https://oci.corp.test"},
			{"tf.corp.test", "buf.corp.test", "oci.corp.test"}, {"BUF.Corp.test"}} {
			c := Discover(nil, environment(nil), t.TempDir())
			c.Credentials(auth.Read(t.TempDir(), environment(nil)))
			c.Trust(trust)
			index, known := c.For(ecosystem, packages[ecosystem])
			got = append(got, fmt.Sprintf("%s %v: %s %v", ecosystem, trust, index, known))
		}
	}
	if want := strings.TrimSpace(registryHostWant); strings.Join(got, "\n") != want {
		t.Errorf("known:\n%s", strings.Join(got, "\n"))
	}
}

const registryHostWant = `
terraform-module []: https://tf.corp.test false
terraform-module [https://tf.corp.test https://buf.corp.test https://oci.corp.test]: https://tf.corp.test true
terraform-module [tf.corp.test buf.corp.test oci.corp.test]: https://tf.corp.test true
terraform-module [BUF.Corp.test]: https://tf.corp.test false
buf []: https://buf.corp.test false
buf [https://tf.corp.test https://buf.corp.test https://oci.corp.test]: https://buf.corp.test true
buf [tf.corp.test buf.corp.test oci.corp.test]: https://buf.corp.test false
buf [BUF.Corp.test]: https://buf.corp.test true
oci []: https://oci.corp.test false
oci [https://tf.corp.test https://buf.corp.test https://oci.corp.test]: https://oci.corp.test true
oci [tf.corp.test buf.corp.test oci.corp.test]: https://oci.corp.test true
oci [BUF.Corp.test]: https://oci.corp.test false
`
