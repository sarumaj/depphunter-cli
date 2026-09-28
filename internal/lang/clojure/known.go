package clojure

import "strings"

// clojureStd are the namespaces org.clojure/clojure itself ships (Clojure 1.12). A key
// ending in "." owns the namespaces under it.
//
// Implements: REQ-CLOJURE-005
var clojureStd = map[string]bool{
	"clojure.core": true, "clojure.core.protocols": true, "clojure.core.reducers": true,
	"clojure.core.server": true, "clojure.data": true, "clojure.datafy": true, "clojure.edn": true,
	"clojure.inspector": true, "clojure.instant": true, "clojure.java.basis": true,
	"clojure.java.browse": true, "clojure.java.browse-ui": true, "clojure.java.io": true,
	"clojure.java.javadoc": true, "clojure.java.process": true, "clojure.java.shell": true,
	"clojure.main": true, "clojure.math": true, "clojure.pprint": true, "clojure.reflect": true,
	"clojure.repl": true, "clojure.repl.deps": true, "clojure.set": true, "clojure.stacktrace": true,
	"clojure.string": true, "clojure.template": true, "clojure.test": true, "clojure.test.junit": true,
	"clojure.test.tap": true, "clojure.uuid": true, "clojure.walk": true, "clojure.xml": true,
	"clojure.zip": true,
}

// cljsNotStd are cljs.* namespaces ClojureScript does not ship.
var cljsNotStd = map[string]string{
	"cljs.core.async": "org.clojure:core.async",
	"cljs-http":       "cljs-http:cljs-http",
}

// contribExceptions are Clojure contrib namespaces whose artifact is not
// org.clojure:<second>.<third> (clojure.data.json is org.clojure:data.json).
var contribExceptions = map[string]string{
	"clojure.spec.alpha":         "org.clojure:spec.alpha",
	"clojure.test.generative":    "org.clojure:test.generative",
	"clojure.spec.gen.alpha":     "org.clojure:spec.alpha",
	"clojure.core.specs.alpha":   "org.clojure:core.specs.alpha",
	"clojure.spec.test.alpha":    "org.clojure:spec.alpha",
	"clojure.tools.analyzer.jvm": "org.clojure:tools.analyzer.jvm",
	"clojure.tools.build":        "io.github.clojure:tools.build",
	"clojure.tools.deps.cli":     "org.clojure:tools.deps.cli",
	"clojure.tools.reader.edn":   "org.clojure:tools.reader",
	"clojure.core.cache.wrapped": "org.clojure:core.cache",
	"clojure.test.check":         "org.clojure:test.check",
	"clojure.math.combinatorics": "org.clojure:math.combinatorics",
	"clojure.math.numeric-tower": "org.clojure:math.numeric-tower",
	"clojure.java.jdbc":          "org.clojure:java.jdbc",
	"clojure.java.classpath":     "org.clojure:java.classpath",
	"clojure.java.data":          "org.clojure:java.data",
}

// contribFamilies are the second segments of Clojure contrib libraries' namespaces
// (clojure.core.async, clojure.data.json, clojure.tools.cli...).
var contribFamilies = map[string]bool{"core": true, "data": true, "tools": true, "java": true, "math": true, "algo": true}

// contrib names the artifact of a clojure.* namespace outside org.clojure/clojure:
// org.clojure:<a>.<b> for clojure.<a>.<b>..., with the exceptions above.
func contrib(ns string) string {
	for k := ns; k != ""; k = parentNS(k) {
		if a, ok := contribExceptions[k]; ok {
			return a
		}
	}
	segments := strings.Split(ns, ".")
	if len(segments) < 3 || segments[0] != "clojure" || !contribFamilies[segments[1]] {
		return ""
	}
	return "org.clojure:" + segments[1] + "." + segments[2]
}

// knownArtifacts maps namespace prefixes of popular libraries to their artifact
// (group:artifact), where the heuristics against the declared dependencies cannot
// connect them or nothing declares them. Longest prefix wins.
//
// Implements: REQ-CLOJURE-007
var knownArtifacts = map[string]string{
	"ring.util": "ring:ring-core", "ring.middleware": "ring:ring-core", "ring.core": "ring:ring-core",
	"ring.websocket": "ring:ring-core", "ring.adapter.jetty": "ring:ring-jetty-adapter",
	"ring.adapter.jetty9": "info.sunng:ring-jetty9-adapter", "ring.mock": "ring:ring-mock",
	"ring.middleware.json": "ring:ring-json", "ring.middleware.defaults": "ring:ring-defaults",
	"ring.middleware.anti-forgery": "ring:ring-anti-forgery", "ring.middleware.cors": "ring-cors:ring-cors",
	"ring.middleware.gzip": "amalloy:ring-gzip-middleware", "ring.util.http-response": "metosin:ring-http-response",
	"ring.util.http-status": "metosin:ring-http-response", "ring.logger": "ring-logger:ring-logger",
	"ring.middleware.oauth2": "ring-oauth2:ring-oauth2", "ring.middleware.session.cookie": "ring:ring-core",
	"compojure": "compojure:compojure", "cheshire": "cheshire:cheshire",
	"next.jdbc": "com.github.seancorfield:next.jdbc", "honey.sql": "com.github.seancorfield:honeysql",
	"honeysql": "honeysql:honeysql", "reagent": "reagent:reagent", "re-frame": "re-frame:re-frame",
	"day8.re-frame.http-fx": "day8.re-frame:http-fx", "day8.re-frame.test": "day8.re-frame:test",
	"day8.re-frame.tracing": "day8.re-frame:tracing", "day8.re-frame-10x": "day8.re-frame:re-frame-10x",
	"malli": "metosin:malli", "muuntaja": "metosin:muuntaja",
	"jsonista": "metosin:jsonista", "spec-tools": "metosin:spec-tools", "sieppari": "metosin:sieppari",
	"taoensso.timbre": "com.taoensso:timbre", "taoensso.encore": "com.taoensso:encore",
	"taoensso.nippy": "com.taoensso:nippy", "taoensso.carmine": "com.taoensso:carmine",
	"taoensso.sente": "com.taoensso:sente", "taoensso.tempura": "com.taoensso:tempura",
	"taoensso.tufte": "com.taoensso:tufte", "taoensso.truss": "com.taoensso:truss",
	"integrant.core": "integrant:integrant", "integrant.repl": "integrant:repl",
	"mount.core": "mount:mount", "hiccup": "hiccup:hiccup", "hiccup2": "hiccup:hiccup",
	"clj-http": "clj-http:clj-http", "org.httpkit": "http-kit:http-kit", "aleph": "aleph:aleph",
	"manifold": "manifold:manifold", "byte-streams": "org.clj-commons:byte-streams",
	"buddy.core": "buddy:buddy-core", "buddy.sign": "buddy:buddy-sign", "buddy.auth": "buddy:buddy-auth",
	"buddy.hashers": "buddy:buddy-hashers", "environ.core": "environ:environ",
	"schema": "prismatic:schema", "plumbing": "prismatic:plumbing", "medley.core": "medley:medley",
	"clj-time": "clj-time:clj-time", "tick": "tick:tick", "java-time": "clojure.java-time:clojure.java-time",
	"camel-snake-kebab": "camel-snake-kebab:camel-snake-kebab", "potemkin": "potemkin:potemkin",
	"clj-yaml": "clj-commons:clj-yaml", "instaparse": "instaparse:instaparse", "selmer": "selmer:selmer",
	"hugsql": "com.layerware:hugsql", "migratus": "migratus:migratus", "ragtime": "ragtime:ragtime",
	"com.stuartsierra.component":  "com.stuartsierra:component",
	"com.stuartsierra.dependency": "com.stuartsierra:dependency",
	"kaocha":                      "lambdaisland:kaocha", "expound": "expound:expound", "sci": "org.babashka:sci",
	"babashka.fs": "babashka:fs", "babashka.process": "babashka:process",
	"babashka.http-client": "org.babashka:http-client", "babashka.cli": "org.babashka:cli",
	"rewrite-clj": "rewrite-clj:rewrite-clj", "cognitect.transit": "com.cognitect:transit-clj",
	"cognitect.aws": "com.cognitect.aws:api", "promesa": "funcool:promesa", "cuerdas": "funcool:cuerdas",
	"uix": "com.pitch:uix.core", "rum": "rum:rum", "om": "org.omcljs:om", "secretary": "secretary:secretary",
	"cljs-http": "cljs-http:cljs-http", "ajax": "cljs-ajax:cljs-ajax", "datascript": "datascript:datascript",
	"clj-kondo": "clj-kondo:clj-kondo", "nrepl": "nrepl:nrepl", "cider.nrepl": "cider:cider-nrepl",
	"methodical": "methodical:methodical", "toucan2": "io.github.camsaul:toucan2", "toucan": "toucan:toucan",
	"clj-commons.byte-streams": "org.clj-commons:byte-streams", "flatland.ordered": "org.flatland:ordered",
	"lambdaisland.uri": "lambdaisland:uri", "lambdaisland.deep-diff2": "lambdaisland:deep-diff2",
	"edamame": "borkdude:edamame", "clj-commons.digest": "org.clj-commons:digest",
	"mockery": "mockery:mockery", "metosin.malli": "metosin:malli", "fipp": "fipp:fipp",
	"puget": "mvxcvi:puget", "clj-commons.format": "org.clj-commons:pretty", "clj-commons.ansi": "org.clj-commons:pretty",
	"io.pedestal": "io.pedestal:pedestal.service", "yada": "yada:yada", "bidi": "bidi:bidi",
	"liberator": "liberator:liberator", "luminus": "luminus:luminus", "datomic.api": "com.datomic:datomic-free",
	"datomic.client.api": "com.datomic:client-api", "xtdb.api": "com.xtdb:xtdb-core",
	"clojure.tools.logging": "org.clojure:tools.logging", "clojure.core.async": "org.clojure:core.async",
	"cljs.core.async": "org.clojure:core.async", "shadow.cljs": "thheller:shadow-cljs",
	"figwheel": "com.bhauman:figwheel-main", "devcards": "devcards:devcards",
	"environ": "environ:environ", "clj-jwt": "clj-jwt:clj-jwt", "postal": "com.draines:postal",
	"clj-pdf": "clj-pdf:clj-pdf", "dk.ative.docjure": "dk.ative:docjure", "clj-commons.slingshot": "slingshot:slingshot",
	"slingshot": "slingshot:slingshot", "throttler": "throttler:throttler", "overtone.at-at": "overtone:at-at",
	"clojurewerkz.quartzite": "clojurewerkz:quartzite", "monger": "com.novemberain:monger",
	"langohr": "com.novemberain:langohr", "clj-commons.primitive-math": "org.clj-commons:primitive-math",
	"hato": "hato:hato", "martian": "com.github.oliyh:martian", "lacinia": "com.walmartlabs:lacinia",
	"com.walmartlabs.lacinia": "com.walmartlabs:lacinia", "pathom": "com.wsscode:pathom",
	"com.wsscode.pathom3": "com.wsscode:pathom3", "cljfmt": "dev.weavejester:cljfmt",
	"clojure-csv": "clojure-csv:clojure-csv", "zprint": "zprint:zprint", "jonase.eastwood": "jonase:eastwood",
	"eftest": "eftest:eftest", "orchestra": "orchestra:orchestra", "net.cgrand.xforms": "net.cgrand:xforms",
	"net.cgrand.enlive-html": "enlive:enlive", "clj-commons.fs": "clj-commons:fs", "me.raynes.fs": "me.raynes:fs",
	"superstring": "superstring:superstring", "clj-antlr": "clj-antlr:clj-antlr", "gniazdo": "stylefruits:gniazdo",
	"mb.hawk": "io.github.metabase:hawk", "com.climate.claypoole": "org.clj-commons:claypoole",
}

// babashkaBuiltins are the namespaces the bb binary itself includes, for scripts that
// require them without declaring anything (by prefix; a key ending in "." owns the
// namespaces under it).
//
// Implements: REQ-CLOJURE-005
var babashkaBuiltins = []string{
	"babashka.", "cheshire.core", "clojure.data.csv", "clojure.data.xml", "clojure.tools.cli",
	"clj-yaml.core", "cognitect.transit", "selmer.", "hiccup.", "hiccup2.", "rewrite-clj.",
	"bencode.core", "borkdude.deflet", "taoensso.timbre", "org.httpkit.", "clojure.core.async",
	"clojure.core.match", "clojure.test.check", "edamame.core", "sci.core", "clojure.data.json",
	"clojure.tools.logging", "nextjournal.markdown", "clojure.core.rrb-vector",
}

// jsBuiltins are Node's own modules, which a ClojureScript file for Node requires by
// string ("fs", "node:path").
var jsBuiltins = map[string]bool{
	"assert": true, "buffer": true, "child_process": true, "cluster": true, "crypto": true, "dgram": true,
	"dns": true, "events": true, "fs": true, "http": true, "http2": true, "https": true, "net": true,
	"os": true, "path": true, "perf_hooks": true, "process": true, "querystring": true, "readline": true,
	"stream": true, "string_decoder": true, "timers": true, "tls": true, "tty": true, "url": true,
	"util": true, "v8": true, "vm": true, "worker_threads": true, "zlib": true,
}

func parentNS(ns string) string {
	if i := strings.LastIndexByte(ns, '.'); i >= 0 {
		return ns[:i]
	}
	return ""
}

// known is the table's artifact for a namespace, by its longest listed prefix.
func known(ns string) string {
	for k := ns; k != ""; k = parentNS(k) {
		if a, ok := knownArtifacts[k]; ok {
			return a
		}
	}
	if ns == "reitit" {
		return "metosin:reitit"
	}
	// reitit.<x> is metosin/reitit-<x>, taoensso.<x> is com.taoensso/<x>.
	segments := strings.Split(ns, ".")
	if len(segments) >= 2 {
		switch segments[0] {
		case "reitit":
			return "metosin:reitit-" + segments[1]
		case "taoensso":
			return "com.taoensso:" + segments[1]
		case "lambdaisland":
			return "lambdaisland:" + segments[1]
		case "buddy":
			return "buddy:buddy-" + segments[1]
		}
	}
	return ""
}
