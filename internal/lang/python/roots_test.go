package python

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// unresolved is what an import no root finds becomes: a distribution nothing declares.
func unresolved(name string) lang.Target {
	return lang.Target{Ecosystem: "pypi", Package: name, Unresolved: true}
}

// pythonPathOnly is a Getenv giving PYTHONPATH and nothing else.
func pythonPathOnly(pythonPath func() string) func(string) string {
	return func(key string) string {
		if key == "PYTHONPATH" {
			return pythonPath()
		}
		return ""
	}
}

// Each fixture holds a module only the root one source declares reaches: without
// that source, every import in it would be an undeclared distribution.
//
// Verifies: REQ-PY-016
func TestConfiguredImportRoots(t *testing.T) {
	outside := langtest.Write(t, map[string]string{"outsidemod.py": ""})
	for _, c := range []struct {
		name  string
		files map[string]string
		// pythonPath is the process's PYTHONPATH, given the repository root.
		pythonPath func(root string) string
		imports    map[string]map[string]lang.Target
	}{
		{
			name: "the PYTHONPATH environment variable",
			files: map[string]string{
				"libs/mylib.py":            "",
				"third_party/pkg/thing.py": "",
				"app/main.py":              "import mylib\nimport thing\nimport outsidemod\n",
			},
			pythonPath: func(root string) string {
				return strings.Join([]string{"libs", filepath.Join(root, "third_party", "pkg"), outside}, string(os.PathListSeparator))
			},
			imports: map[string]map[string]lang.Target{"app/main.py": {
				"mylib":      {Local: "libs/mylib.py"},
				"thing":      {Local: "third_party/pkg/thing.py"},
				"outsidemod": unresolved("outsidemod"), // outside the repository
			}},
		},
		{
			name: ".env",
			files: map[string]string{
				".env": "# local settings\nSECRET_TOKEN=hunter2-do-not-leak\nexport BASE=${workspaceFolder}\n" +
					"PYTHONPATH=\"${BASE}/libs:${PWD}/shared:../outside:${UNSET:-defaults}\" # comment\n",
				"libs/mylib.py":                  "",
				"shared/common.py":               "",
				"defaults/fallback.py":           "",
				"app/main.py":                    "import mylib\nimport common\nimport fallback\n",
				"deploy/.env":                    "PYTHONPATH='conf'\n",
				"deploy/conf/settings_module.py": "",
				"deploy/run/start.py":            "import settings_module\n",
			},
			imports: map[string]map[string]lang.Target{
				"app/main.py": {
					"mylib":    {Local: "libs/mylib.py"},
					"common":   {Local: "shared/common.py"},
					"fallback": {Local: "defaults/fallback.py"},
				},
				"deploy/run/start.py": {"settings_module": {Local: "deploy/conf/settings_module.py"}},
			},
		},
		{
			name: ".vscode/settings.json",
			files: map[string]string{
				".vscode/settings.json": `{
	// Pylance
	"python.analysis.extraPaths": ["${workspaceFolder}/vendor_src", "../outside",],
	/* the older language server */
	"python.autoComplete.extraPaths": ["legacy"],
	"python.envFile": "${workspaceFolder}/config/dev.env",
	"terminal.integrated.env.linux": {"PYTHONPATH": "${workspaceFolder}/tools:${env:PYTHONPATH}"},
	"terminal.integrated.env.windows": {"PYTHONPATH": "wintools;${env:PYTHONPATH}"},
}`,
				"config/dev.env":      "PYTHONPATH=../plugins\n",
				"vendor_src/vend.py":  "",
				"legacy/old.py":       "",
				"tools/toolkit.py":    "",
				"wintools/wintool.py": "",
				"plugins/plugin_a.py": "",
				"app/main.py":         "import vend\nimport old\nimport toolkit\nimport wintool\nimport plugin_a\n",
			},
			imports: map[string]map[string]lang.Target{"app/main.py": {
				"vend":     {Local: "vendor_src/vend.py"},
				"old":      {Local: "legacy/old.py"},
				"toolkit":  {Local: "tools/toolkit.py"},
				"wintool":  {Local: "wintools/wintool.py"},
				"plugin_a": {Local: "plugins/plugin_a.py"},
			}},
		},
		{
			name: "pyrightconfig.json execution environments",
			files: map[string]string{
				"pyrightconfig.json": `{
	// comments are allowed
	"extraPaths": ["common"],
	"executionEnvironments": [{"root": "services/api", "extraPaths": ["shared/py"]}]
}`,
				"common/commonthing.py":    "",
				"services/api/handlers.py": "",
				"services/api/app/main.py": "import handlers\nimport sharedthing\nimport commonthing\n",
				"shared/py/sharedthing.py": "",
				"tools/run.py":             "import handlers\nimport sharedthing\nimport commonthing\n",
			},
			imports: map[string]map[string]lang.Target{
				"services/api/app/main.py": {
					"handlers":    {Local: "services/api/handlers.py"},
					"sharedthing": {Local: "shared/py/sharedthing.py"},
					"commonthing": {Local: "common/commonthing.py"},
				},
				// Outside the execution environment only the top-level paths apply.
				"tools/run.py": {
					"handlers":    unresolved("handlers"),
					"sharedthing": unresolved("sharedthing"),
					"commonthing": {Local: "common/commonthing.py"},
				},
			},
		},
		{
			name: "[tool.pyright] and [tool.basedpyright] in pyproject.toml",
			files: map[string]string{
				"pyproject.toml":      "[tool.pyright]\nextraPaths = [\"typed\"]\n\n[tool.basedpyright]\nexecutionEnvironments = [{ root = \"web\", extraPaths = [\"webshared\"] }]\n",
				"typed/typedmod.py":   "",
				"webshared/widget.py": "",
				"web/views/page.py":   "import typedmod\nimport widget\n",
			},
			imports: map[string]map[string]lang.Target{"web/views/page.py": {
				"typedmod": {Local: "typed/typedmod.py"},
				"widget":   {Local: "webshared/widget.py"},
			}},
		},
		{
			name: "pytest pythonpath",
			files: map[string]string{
				"backend/pytest.ini":              "[pytest]\npythonpath = pylib\n    extra ; comment\n",
				"backend/pylib/mylib.py":          "",
				"backend/extra/extramod.py":       "",
				"backend/tests/unit/test_x.py":    "import mylib\nimport extramod\n",
				"api/pyproject.toml":              "[tool.pytest.ini_options]\npythonpath = [\"helpers\"]\n",
				"api/helpers/helper.py":           "",
				"api/tests/deep/test_api.py":      "import helper\n",
				"cli/setup.cfg":                   "[tool:pytest]\npythonpath = support\n",
				"cli/support/clisupport.py":       "",
				"cli/tests/deep/test_cli.py":      "import clisupport\n",
				"worker/tox.ini":                  "[tox]\nenvlist = py312\n\n[pytest]\npythonpath = fixtures_lib\n",
				"worker/fixtures_lib/fixtures.py": "",
				"worker/tests/deep/test_w.py":     "import fixtures\n",
			},
			imports: map[string]map[string]lang.Target{
				"backend/tests/unit/test_x.py": {"mylib": {Local: "backend/pylib/mylib.py"}, "extramod": {Local: "backend/extra/extramod.py"}},
				"api/tests/deep/test_api.py":   {"helper": {Local: "api/helpers/helper.py"}},
				"cli/tests/deep/test_cli.py":   {"clisupport": {Local: "cli/support/clisupport.py"}},
				"worker/tests/deep/test_w.py":  {"fixtures": {Local: "worker/fixtures_lib/fixtures.py"}},
			},
		},
		{
			name: "mypy_path",
			files: map[string]string{
				"config/mypy.ini":          "[mypy]\nmypy_path = $MYPY_CONFIG_FILE_DIR/stubs, typings\n",
				"config/stubs/stubbed.pyi": "",
				"typings/typedlib.py":      "",
				"svc/pyproject.toml":       "[tool.mypy]\nmypy_path = \"svc/mypy_extra\"\n",
				"svc/mypy_extra/extra.py":  "",
				"app/main.py":              "import stubbed\nimport typedlib\nimport extra\n",
			},
			imports: map[string]map[string]lang.Target{"app/main.py": {
				"stubbed":  {Local: "config/stubs/stubbed.pyi"},
				"typedlib": {Local: "typings/typedlib.py"}, // relative to the working directory
				"extra":    {Local: "svc/mypy_extra/extra.py"},
			}},
		},
		{
			name: "setuptools package-dir and where",
			files: map[string]string{
				"one/pyproject.toml":             "[tool.setuptools]\npackage-dir = {\"\" = \"lib\"}\n",
				"one/lib/libone/__init__.py":     "",
				"two/setup.cfg":                  "[options]\npackage_dir =\n    =code\n\n[options.packages.find]\nwhere = code\n",
				"two/code/libtwo/__init__.py":    "",
				"three/pyproject.toml":           "[tool.setuptools.packages.find]\nwhere = [\"sources\"]\n\n[tool.setuptools.package-dir]\n\"named.pkg\" = \"elsewhere/named/pkg\"\n",
				"three/sources/libthree.py":      "",
				"three/elsewhere/named/pkg/m.py": "",
				"four/setup.py":                  "from setuptools import setup\nsetup(package_dir={'': 'python'})\n",
				"four/python/libfour.py":         "",
				"scripts/run.py":                 "import libone\nimport libtwo\nimport libthree\nimport named.pkg.m\nimport libfour\n",
			},
			imports: map[string]map[string]lang.Target{"scripts/run.py": {
				"libone":      {Local: "one/lib/libone"},
				"libtwo":      {Local: "two/code/libtwo"},
				"libthree":    {Local: "three/sources/libthree.py"},
				"named.pkg.m": {Local: "three/elsewhere/named/pkg/m.py"},
				"libfour":     {Local: "four/python/libfour.py"},
			}},
		},
		{
			name: "Poetry, Hatch, PDM and maturin source directories",
			files: map[string]string{
				"poetry/pyproject.toml":           "[tool.poetry]\nname = \"x\"\npackages = [{ include = \"poetrylib\", from = \"python\" }]\n",
				"poetry/python/poetrylib.py":      "",
				"hatch/pyproject.toml":            "[tool.hatch.build.targets.wheel]\npackages = [\"pkgs/hatchlib\"]\n\n[tool.hatch.build]\nsources = [\"alt\"]\n",
				"hatch/pkgs/hatchlib/__init__.py": "",
				"hatch/alt/altlib.py":             "",
				"pdm/pyproject.toml":              "[tool.pdm.build]\npackage-dir = \"pysrc\"\n",
				"pdm/pysrc/pdmlib.py":             "",
				"rust/pyproject.toml":             "[tool.maturin]\npython-source = \"py\"\n",
				"rust/py/rustlib.py":              "",
				"scripts/run.py":                  "import poetrylib\nimport hatchlib\nimport altlib\nimport pdmlib\nimport rustlib\n",
			},
			imports: map[string]map[string]lang.Target{"scripts/run.py": {
				"poetrylib": {Local: "poetry/python/poetrylib.py"},
				"hatchlib":  {Local: "hatch/pkgs/hatchlib"},
				"altlib":    {Local: "hatch/alt/altlib.py"},
				"pdmlib":    {Local: "pdm/pysrc/pdmlib.py"},
				"rustlib":   {Local: "rust/py/rustlib.py"},
			}},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := langtest.Write(t, c.files)
			plugin := Plugin{}
			if c.pythonPath != nil {
				plugin.Getenv = pythonPathOnly(func() string { return c.pythonPath(root) })
			}
			results := langtest.Analyze(t, plugin, root)
			for file, want := range c.imports {
				langtest.CheckImports(t, results[file], want)
			}
			// Without the source, the same imports are undeclared distributions.
			bare := langtest.Analyze(t, Plugin{}, langtest.Write(t, withoutConfiguration(c.files)))
			for file, want := range c.imports {
				got := langtest.Imports(t, bare[file])
				for spec := range want {
					if got[spec].Local != "" {
						t.Errorf("%s: %s resolves to %s with no configuration", file, spec, got[spec].Local)
					}
				}
			}
		})
	}
}

// withoutConfiguration is files less the ones that declare import roots.
func withoutConfiguration(files map[string]string) map[string]string {
	out := map[string]string{}
	for name, content := range files {
		if (strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".pyi")) && !strings.HasSuffix(name, "setup.py") {
			out[name] = content
		}
	}
	return out
}

// A sub-project's own modules shadow a configured root's (for every file, as they
// did before), and a configured root's shadow the repository root's; the notes say
// what added each root.
//
// Verifies: REQ-PY-003, REQ-PY-016
func TestImportRootOrder(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"sub/pyproject.toml":       "[project]\nname = \"sub\"\n",
		"sub/mod.py":               "",
		"libs/mod.py":              "",
		"mod.py":                   "",
		"libs/shared_name.py":      "",
		"shared_name.py":           "",
		"other/deep/b.py":          "import mod\nimport shared_name\nimport both\nimport layered\n",
		".env":                     "PYTHONPATH=libs\n",
		"libs/both.py":             "",
		"envlibs/both.py":          "",
		"deep/pytest.ini":          "[pytest]\npythonpath = deeplibs\n",
		"deep/deeplibs/layered.py": "",
		"libs/layered.py":          "",
	})
	plugin := Plugin{Getenv: pythonPathOnly(func() string { return "envlibs" })}
	results := langtest.Analyze(t, plugin, root)
	langtest.CheckImports(t, results["other/deep/b.py"], map[string]lang.Target{
		"mod":         {Local: "sub/mod.py"},
		"shared_name": {Local: "libs/shared_name.py"},
		"both":        {Local: "envlibs/both.py"},          // the process's PYTHONPATH first
		"layered":     {Local: "deep/deeplibs/layered.py"}, // then deeper files first
	})

	r, err := plugin.Resolver(root, langtest.Files(t, root))
	if err != nil {
		t.Fatal(err)
	}
	var directories []string
	for _, importRoot := range r.(*resolver).roots {
		directories = append(directories, importRoot.directory)
	}
	if want := []string{"sub", "envlibs", "deep/deeplibs", "libs", "."}; !slices.Equal(directories, want) {
		t.Errorf("roots %v, want %v", directories, want)
	}
	var messages []string
	for _, note := range r.(lang.Noter).Notes() {
		messages = append(messages, note.File+": "+note.Code+": "+note.Message)
	}
	if want := []string{
		": import-root: PYTHONPATH adds envlibs to the import roots",
		".env: import-root: PYTHONPATH in .env adds libs to the import roots",
		"deep/pytest.ini: import-root: pytest pythonpath adds deep/deeplibs to the import roots",
	}; !slices.Equal(messages, want) {
		t.Errorf("notes %q, want %q", messages, want)
	}
}

// A .env file holds secrets: nothing but its PYTHONPATH may reach what the plugin
// extracts, resolves or notes.
//
// Verifies: REQ-PY-016
func TestDotenvSecretsStayPrivate(t *testing.T) {
	const secret = "sk-live-0123456789abcdef"
	root := langtest.Write(t, map[string]string{
		".env":          "API_KEY=" + secret + "\nPASSWORD='" + secret + "'\nPYTHONPATH=libs\n",
		"libs/mylib.py": "",
		"app/main.py":   "import mylib\n",
	})
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["app/main.py"], map[string]lang.Target{"mylib": {Local: "libs/mylib.py"}})
	r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
	if err != nil {
		t.Fatal(err)
	}
	for what, value := range map[string]any{"the extraction": results, "the notes": r.(lang.Noter).Notes()} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), secret) {
			t.Errorf("%s carries the secret: %s", what, data)
		}
	}
}

// Verifies: REQ-PY-016
func TestDotenvValue(t *testing.T) {
	predefined := map[string]string{"workspaceFolder": "/w", "PWD": "/p"}
	for _, c := range []struct {
		name, data, want string
		found            bool
	}{
		{"unquoted with a comment", "PYTHONPATH=a:b # note\n", "a:b", true},
		{"a # inside a value", "PYTHONPATH=a#b\n", "a#b", true},
		{"export", "export PYTHONPATH=src\n", "src", true},
		{"single quotes are literal", "A=x\nPYTHONPATH='${A}/lib'\n", "${A}/lib", true},
		{"double quotes expand", "A=x\nPYTHONPATH=\"${A}/lib\"\n", "x/lib", true},
		{"unquoted expands", "PYTHONPATH=${workspaceFolder}/lib:${PWD}/other\n", "/w/lib:/p/other", true},
		{"a default", "PYTHONPATH=${MISSING:-fallback}\n", "fallback", true},
		{"an unknown name is empty", "PYTHONPATH=${MISSING}:lib\n", ":lib", true},
		{"a later key is not yet set", "PYTHONPATH=${B}lib\nB=x\n", "lib", true},
		{"the last one wins", "PYTHONPATH=a\nPYTHONPATH=b\n", "b", true},
		{"spaces around =", "PYTHONPATH = lib \n", "lib", true},
		{"a quoted value spans lines", "NOTE=\"one\ntwo\"\nPYTHONPATH=lib\n", "lib", true},
		{"escaped quotes", `PYTHONPATH="a\"b"` + "\n", `a"b`, true},
		{"a commented-out line", "#PYTHONPATH=lib\n", "", false},
		{"exported is a key too", "exported=1\n", "", false},
		{"CRLF", "PYTHONPATH=lib\r\n", "lib", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, found := dotenvValue(c.data, "PYTHONPATH", predefined)
			if got != c.want || found != c.found {
				t.Errorf("got %q %v, want %q %v", got, found, c.want, c.found)
			}
		})
	}
}

// Verifies: REQ-PY-016
func TestSplitPathList(t *testing.T) {
	for value, want := range map[string][]string{
		"a:b":                  {"a", "b"},
		`C:\repo\lib;D:/other`: {`C:\repo\lib`, "D:/other"},
		`C:\repo\lib:C:/other`: {`C:\repo\lib`, "C:/other"},
		"/abs/a:rel":           {"/abs/a", "rel"},
		"single":               {"single"},
	} {
		if got := splitPathList(value); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", value, got, want)
		}
	}
}

// Verifies: REQ-PY-016
func TestRankOf(t *testing.T) {
	for file, want := range map[string]int{
		".env": 0, ".vscode/settings.json": 0, "sub/.vscode/settings.json": 1,
		"sub/pytest.ini": 1, "a/b/mypy.ini": 2,
	} {
		if got := rankOf(file); got != want {
			t.Errorf("%s: %d, want %d", file, got, want)
		}
	}
	if rankOf("") <= rankOf("a/b/c/d/e/f/.env") {
		t.Error("the process's PYTHONPATH does not come first")
	}
}
