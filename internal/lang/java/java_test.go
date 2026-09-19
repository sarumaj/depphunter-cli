package java

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a Maven module (properties, dependencyManagement) and a Gradle module
// with a version catalog.
func analyse(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

func TestResolution(t *testing.T) {
	res := analyse(t)["src/main/java/com/example/app/App.java"]
	langtest.CheckImports(t, res, map[string]lang.Target{
		"import java.util.List":                                       {Ecosystem: "jdk", Package: "java.util"},
		"import javax.swing.JFrame":                                   {Ecosystem: "jdk", Package: "javax.swing"},
		"import javax.servlet.http.HttpServlet":                       {Ecosystem: "maven", Package: "javax.servlet.http", Unresolved: true},
		"import com.example.app.model.User":                           {Local: "src/main/java/com/example/app/model/User.java"},
		"import com.example.app.model.*":                              {Local: "src/main/java/com/example/app/model"},
		"import static com.example.app.util.Strings.trim":             {Local: "src/main/java/com/example/app/util/Strings.java"},
		"import com.example.generated.Gen":                            {},
		"import org.springframework.context.ApplicationContext":       {Ecosystem: "maven", Package: "org.springframework", Version: "6.1.0"},
		"import com.fasterxml.jackson.databind.ObjectMapper":          {Ecosystem: "maven", Package: "com.fasterxml.jackson.core", Version: "2.17.0"},
		"import static org.junit.jupiter.api.Assertions.assertEquals": {Ecosystem: "maven", Package: "org.junit.jupiter", Version: "5.10.2"},
		"import okhttp3.OkHttpClient":                                 {Ecosystem: "maven", Package: "com.squareup.okhttp3", Version: "4.12.0"},
		// Packages sharing too little with their groupId come from knownGroups.
		"import com.google.common.collect.Lists": {Ecosystem: "maven", Package: "com.google.guava", Version: "33.0.0-jre"},
		"import org.junit.Test":                  {Ecosystem: "maven", Package: "junit", Version: "4.13.2"},
		"import org.acme.net.Client":             {Local: "lib/src/main/java/org/acme/net/Client.java"},
	})
}

func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyse(t)["src/main/java/com/example/app/App.java"], map[string]string{
		"App": "class", "App.main": "method", "App.helper": "method", "Service": "interface",
		"Mode": "enum", "Mode.weight": "method", "Point": "record",
	})
}
