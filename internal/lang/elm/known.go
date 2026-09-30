package elm

import (
	"cmp"
	"strings"
)

// coreModules are the modules of elm/core. Elm imports Basics, List, Maybe,
// Result, String, Char, Tuple, Debug, Platform, Platform.Cmd and Platform.Sub into
// every module without an import; only an import written out is an edge.
var coreModules = map[string]bool{
	"Array": true, "Basics": true, "Bitwise": true, "Char": true, "Debug": true, "Dict": true, "List": true,
	"Maybe": true, "Platform": true, "Platform.Cmd": true, "Platform.Sub": true, "Process": true,
	"Result": true, "Set": true, "String": true, "Task": true, "Tuple": true,
}

const core = "elm/core"

// coreKernel are the kernel (JavaScript) modules of elm/core, which other kernel
// packages import as Elm.Kernel.<name>.
var coreKernel = map[string]bool{
	"Basics": true, "Bitwise": true, "Char": true, "Debug": true, "JsArray": true, "List": true,
	"Platform": true, "Process": true, "Scheduler": true, "String": true, "Utils": true,
}

// knownModules maps a module, and every module under it, to the package that
// exposes it, for packages widely used enough that a project without them
// installed still gets the right name. A longer entry wins (Html.Styled is
// rtfeldman/elm-css, Html elm/html).
var knownModules = map[string]string{
	"Simple.Animation":               "andrewMacmurray/elm-simple-animation",
	"Color":                          "avh4/elm-color",
	"Debug.Control":                  "avh4/elm-debug-controls",
	"ProgramTest":                    "avh4/elm-program-test",
	"SimulatedEffect":                "avh4/elm-program-test",
	"Particle":                       "BrianHicks/elm-particle",
	"Particle.System":                "BrianHicks/elm-particle",
	"FormatNumber":                   "cuducos/elm-format-number",
	"Graphql":                        "dillonkearns/elm-graphql",
	"Markdown.Block":                 "dillonkearns/elm-markdown",
	"Markdown.Parser":                "dillonkearns/elm-markdown",
	"Markdown.Renderer":              "dillonkearns/elm-markdown",
	"BackendTask":                    "dillonkearns/elm-pages",
	"FatalError":                     "dillonkearns/elm-pages",
	"Pages":                          "dillonkearns/elm-pages",
	"Array.Extra":                    "elm-community/array-extra",
	"Basics.Extra":                   "elm-community/basics-extra",
	"Dict.Extra":                     "elm-community/dict-extra",
	"Html.Attributes.Extra":          "elm-community/html-extra",
	"Html.Events.Extra":              "elm-community/html-extra",
	"Html.Extra":                     "elm-community/html-extra",
	"Json.Decode.Extra":              "elm-community/json-extra",
	"Json.Encode.Extra":              "elm-community/json-extra",
	"List.Extra":                     "elm-community/list-extra",
	"Maybe.Extra":                    "elm-community/maybe-extra",
	"Random.Extra":                   "elm-community/random-extra",
	"Result.Extra":                   "elm-community/result-extra",
	"String.Extra":                   "elm-community/string-extra",
	"Benchmark":                      "elm-explorations/benchmark",
	"Math.Matrix4":                   "elm-explorations/linear-algebra",
	"Math.Vector2":                   "elm-explorations/linear-algebra",
	"Math.Vector3":                   "elm-explorations/linear-algebra",
	"Math.Vector4":                   "elm-explorations/linear-algebra",
	"Markdown":                       "elm-explorations/markdown",
	"Expect":                         "elm-explorations/test",
	"Fuzz":                           "elm-explorations/test",
	"Shrink":                         "elm-explorations/test",
	"Test":                           "elm-explorations/test",
	"WebGL":                          "elm-explorations/webgl",
	"Browser":                        "elm/browser",
	"Browser.Dom":                    "elm/browser",
	"Browser.Events":                 "elm/browser",
	"Browser.Navigation":             "elm/browser",
	"Bytes":                          "elm/bytes",
	"File":                           "elm/file",
	"Html":                           "elm/html",
	"Html.Keyed":                     "elm/html",
	"Html.Lazy":                      "elm/html",
	"Http":                           "elm/http",
	"Json.Decode":                    "elm/json",
	"Json.Encode":                    "elm/json",
	"Parser":                         "elm/parser",
	"Parser.Advanced":                "elm/parser",
	"Elm.Constraint":                 "elm/project-metadata-utils",
	"Elm.Docs":                       "elm/project-metadata-utils",
	"Elm.License":                    "elm/project-metadata-utils",
	"Elm.Module":                     "elm/project-metadata-utils",
	"Elm.Package":                    "elm/project-metadata-utils",
	"Elm.Project":                    "elm/project-metadata-utils",
	"Elm.Type":                       "elm/project-metadata-utils",
	"Elm.Version":                    "elm/project-metadata-utils",
	"Random":                         "elm/random",
	"Regex":                          "elm/regex",
	"Svg":                            "elm/svg",
	"Svg.Attributes":                 "elm/svg",
	"Svg.Lazy":                       "elm/svg",
	"Time":                           "elm/time",
	"Url":                            "elm/url",
	"Url.Builder":                    "elm/url",
	"Url.Parser":                     "elm/url",
	"VirtualDom":                     "elm/virtual-dom",
	"Keyboard.Event":                 "Gizra/elm-keyboard-event",
	"Html.Parser":                    "hecrj/html-parser",
	"Material.Icons":                 "icidasset/elm-material-icons",
	"Review.Fix":                     "jfmengels/elm-review",
	"Review.ModuleNameLookupTable":   "jfmengels/elm-review",
	"Review.Project":                 "jfmengels/elm-review",
	"Review.Rule":                    "jfmengels/elm-review",
	"Review.Test":                    "jfmengels/elm-review",
	"NoConfusingPrefixOperator":      "jfmengels/elm-review-common",
	"NoDeprecated":                   "jfmengels/elm-review-common",
	"NoExposingEverything":           "jfmengels/elm-review-common",
	"NoImportingEverything":          "jfmengels/elm-review-common",
	"NoMissingTypeAnnotation":        "jfmengels/elm-review-common",
	"NoMissingTypeAnnotationInLetIn": "jfmengels/elm-review-common",
	"NoMissingTypeExpose":            "jfmengels/elm-review-common",
	"NoPrematureLetComputation":      "jfmengels/elm-review-common",
	"NoSimpleLetBody":                "jfmengels/elm-review-common",
	"NoUnoptimizedRecursion":         "jfmengels/elm-review-common",
	"NoDebug":                        "jfmengels/elm-review-debug",
	"Docs":                           "jfmengels/elm-review-documentation",
	"Simplify":                       "jfmengels/elm-review-simplify",
	"NoMissingSubscriptionsCall":     "jfmengels/elm-review-the-elm-architecture",
	"NoRecursiveUpdate":              "jfmengels/elm-review-the-elm-architecture",
	"NoUselessSubscriptions":         "jfmengels/elm-review-the-elm-architecture",
	"NoUnused":                       "jfmengels/elm-review-unused",
	"Date":                           "justinmimbs/date",
	"Time.Extra":                     "justinmimbs/time-extra",
	"TimeZone":                       "justinmimbs/timezone-data",
	"Http.Detailed":                  "jzxhuang/http-extras",
	"Http.Extras":                    "jzxhuang/http-extras",
	"RemoteData":                     "krisajenkins/remotedata",
	"Animation":                      "mdgriffith/elm-style-animation",
	"Element":                        "mdgriffith/elm-ui",
	"List.Nonempty":                  "mgold/elm-nonempty-list",
	"Codec":                          "miniBill/elm-codec",
	"Round":                          "myrho/elm-round",
	"Json.Decode.Pipeline":           "NoRedInk/elm-json-decode-pipeline",
	"Nri.Ui":                         "NoRedInk/noredink-ui",
	"Css":                            "rtfeldman/elm-css",
	"Html.Styled":                    "rtfeldman/elm-css",
	"Svg.Styled":                     "rtfeldman/elm-css",
	"Iso8601":                        "rtfeldman/elm-iso8601-date-strings",
	"Sort":                           "rtfeldman/elm-sorter-experiment",
	"Sort.Dict":                      "rtfeldman/elm-sorter-experiment",
	"Sort.Set":                       "rtfeldman/elm-sorter-experiment",
	"Elm.Interface":                  "stil4m/elm-syntax",
	"Elm.Parser":                     "stil4m/elm-syntax",
	"Elm.Processing":                 "stil4m/elm-syntax",
	"Elm.RawFile":                    "stil4m/elm-syntax",
	"Elm.Syntax":                     "stil4m/elm-syntax",
	"Elm.Writer":                     "stil4m/elm-syntax",
	"Accessibility":                  "tesk9/accessible-html",
	"Accessibility.Styled":           "tesk9/accessible-html-with-css",
	"Palette.Cubehelix":              "tesk9/palette",
	"Palette.Generative":             "tesk9/palette",
	"Palette.Tango":                  "tesk9/palette",
	"Palette.X11":                    "tesk9/palette",
	"SolidColor":                     "tesk9/palette",
	"TransparentColor":               "tesk9/palette",
}

// knownPackage is the package the longest entry of knownModules covering module
// names, preferring one the project declares: of Html.Styled.Attributes, Html is
// elm/html but Html.Styled rtfeldman/elm-css, and when only elm/html is declared
// Html wins over the longer, undeclared match.
func knownPackage(module string, declared func(string) bool) (name string, ok bool) {
	var first string
	for m := module; m != ""; m = parent(m) {
		packageName, found := knownModules[m]
		if !found {
			continue
		}
		if declared(packageName) {
			return packageName, true
		}
		first = cmp.Or(first, packageName)
	}
	return first, first != ""
}

// parent drops a module's last segment: Html.Styled.Attributes -> Html.Styled.
func parent(module string) string {
	if i := strings.LastIndexByte(module, '.'); i >= 0 {
		return module[:i]
	}
	return ""
}
