package auth

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Which credential is sent when several of this machine's files name one for the
// same host. Nothing decides it but the order the readers run in (readMachine) and
// whether each files over what is there: a bearer and a basic credential of one host
// are kept side by side and the bearer is sent, a later basic one replaces an earlier,
// and the host with its port is asked before the host alone - for bearer tokens
// first, then for basic ones.
//
// Verifies: REQ-AUTH-014, REQ-AUTH-020
func TestCredentialPrecedence(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".npmrc"),
		"//a.test/:_authToken=npm-a\n//c.test:8443/:_authToken=npm-c-port\n//d.test/:_authToken=npm-d\n")
	writeFile(t, filepath.Join(home, ".netrc"),
		"machine a.test login n password a\nmachine b.test login n password b\nmachine c.test login n password c\n"+
			"machine d.test:8443 login n password d-port\nmachine e.test login n password e\n")
	writeFile(t, filepath.Join(home, ".m2", "settings.xml"), `<settings>
  <servers><server><id>b</id><username>mvn</username><password>b</password></server></servers>
  <mirrors><mirror><id>b</id><url>https://b.test/maven</url></mirror></mirrors>
</settings>`)
	writeFile(t, filepath.Join(home, ".composer", "auth.json"),
		`{"bearer": {"e.test": "composer-e"}, "http-basic": {"f.test": {"username": "c", "password": "f"}}}`)
	writeFile(t, filepath.Join(home, ".terraformrc"), "credentials \"f.test\" {\n  token = \"tf-f\"\n}\n")
	c := Read(home, func(string) string { return "" })
	var got []string
	for _, u := range []string{"https://a.test/x", "https://b.test/maven/x", "https://c.test:8443/x", "https://c.test/x",
		"https://d.test:8443/x", "https://d.test/x", "https://e.test/x", "https://f.test/x"} {
		got = append(got, fmt.Sprintf("%s %s", u, authorization(t, c, u)))
	}
	if want := strings.TrimSpace(precedenceWant); strings.Join(got, "\n") != want {
		t.Errorf("precedence:\n%s", strings.Join(got, "\n"))
	}
}

const precedenceWant = `
https://a.test/x Bearer npm-a
https://b.test/maven/x Basic bXZuOmI=
https://c.test:8443/x Bearer npm-c-port
https://c.test/x Basic bjpj
https://d.test:8443/x Bearer npm-d
https://d.test/x Bearer npm-d
https://e.test/x Bearer composer-e
https://f.test/x Bearer tf-f
`
