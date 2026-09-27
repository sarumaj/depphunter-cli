package haskell

import "strings"

// stdPkgs are the packages that come with GHC and cannot be chosen apart from it:
// the compiler's own library and its runtime. They make the hidden haskell-std
// island whether or not build-depends names them. The other packages GHC bundles
// (containers, text, bytestring, mtl, directory, ...) are released on Hackage, are
// declared in build-depends like any other and resolved as Hackage packages.
var stdPkgs = map[string]bool{
	"base": true, "ghc-prim": true, "ghc": true, "template-haskell": true, "integer-gmp": true,
	"ghc-bignum": true, "ghc-boot": true, "ghc-boot-th": true, "ghc-heap": true, "rts": true,
	"ghc-internal": true, "ghc-experimental": true, "integer-simple": true,
}

// moduleTable maps module names to the packages that provide them. A key owns the
// module of its name and every module under it (Data.Map owns Data.Map.Strict); the
// longest key matching a module wins, so Data.ByteString.Base64 (base64-bytestring)
// beats Data.ByteString (bytestring). Where two packages provide one module
// (cryptonite and its fork crypton), the one the project declares is taken.
var moduleTable = map[string][]string{}

func init() {
	for pkgs, mods := range map[string]string{
		"base": `Prelude Control.Applicative Control.Arrow Control.Category Control.Concurrent
			Control.Concurrent.Chan Control.Concurrent.MVar Control.Concurrent.QSem Control.Concurrent.QSemN
			Control.Exception Control.Monad Control.Monad.Fail Control.Monad.Fix Control.Monad.Instances
			Control.Monad.IO.Class Control.Monad.ST Control.Monad.Zip Data.Array.Byte Data.Bifoldable
			Data.Bifoldable1 Data.Bifunctor Data.Bitraversable Data.Bits Data.Bool Data.Char Data.Coerce
			Data.Complex Data.Data Data.Dynamic Data.Either Data.Eq Data.Fixed Data.Foldable Data.Foldable1
			Data.Function Data.Functor Data.IORef Data.Int Data.Ix Data.Kind Data.List Data.Maybe
			Data.Monoid Data.Ord Data.Proxy Data.Ratio Data.STRef Data.Semigroup Data.String
			Data.Traversable Data.Tuple Data.Type Data.Typeable Data.Unique Data.Version Data.Void
			Data.Word Debug.Trace Foreign GHC Numeric System.CPUTime System.Console.GetOpt
			System.Environment System.Exit System.IO System.Info System.Mem System.Posix.Internals
			System.Posix.Types System.Timeout Text.ParserCombinators.ReadP Text.ParserCombinators.ReadPrec
			Text.Printf Text.Read Text.Show Type.Reflection Unsafe.Coerce`,
		"ghc-prim": `GHC.Prim GHC.Types GHC.Classes GHC.CString GHC.Magic GHC.Tuple GHC.PrimopWrappers
			GHC.Prim.Ext GHC.Prim.Panic GHC.Debug`,
		"template-haskell": `Language.Haskell.TH`,
		"ghc":              `GHC.Driver GHC.Hs GHC.Core GHC.Plugins GHC.Tc GHC.Types.Name GHC.Unit GHC.Utils GHC.Data GHC.Parser GHC.Iface GHC.Runtime GHC.HsToCore GHC.Rename GHC.Stg GHC.Cmm GHC.CoreToStg GHC.StgToCmm GHC.Builtin GHC.Settings GHC.SysTools GHC.Linker GHC.Platform GhcPlugins HscTypes DynFlags`,
		"ghc-paths":        `GHC.Paths`,
		"ghc-boot":         `GHC.Data.ShortText GHC.Unit.Database GHC.Serialized GHC.Platform.Host GHC.Settings.Utils`,
		"containers": `Data.Map Data.Set Data.IntMap Data.IntSet Data.Sequence Data.Tree Data.Graph
			Data.Containers.ListUtils`,
		"text":                              `Data.Text`,
		"bytestring":                        `Data.ByteString`,
		"mtl":                               `Control.Monad.State Control.Monad.Reader Control.Monad.Writer Control.Monad.RWS Control.Monad.Except Control.Monad.Error.Class Control.Monad.Cont Control.Monad.Identity Control.Monad.Trans Control.Monad.Accum Control.Monad.Select`,
		"transformers":                      `Control.Monad.Trans.Class Control.Monad.Trans.State Control.Monad.Trans.Reader Control.Monad.Trans.Writer Control.Monad.Trans.RWS Control.Monad.Trans.Except Control.Monad.Trans.Maybe Control.Monad.Trans.Cont Control.Monad.Trans.Identity Control.Monad.Trans.Select Control.Monad.Trans.Accum Data.Functor.Reverse Control.Applicative.Backwards Control.Applicative.Lift`,
		"aeson":                             `Data.Aeson`,
		"aeson-pretty":                      `Data.Aeson.Encode.Pretty`,
		"attoparsec-aeson":                  `Data.Aeson.Parser`,
		"lens-aeson":                        `Data.Aeson.Lens`,
		"unordered-containers":              `Data.HashMap Data.HashSet`,
		"hashable":                          `Data.Hashable`,
		"vector":                            `Data.Vector`,
		"vector-algorithms":                 `Data.Vector.Algorithms`,
		"lens":                              `Control.Lens Data.Text.Lens Data.Map.Lens Numeric.Lens System.FilePath.Lens`,
		"microlens":                         `Lens.Micro`,
		"microlens-platform":                `Lens.Micro.Platform`,
		"microlens-th":                      `Lens.Micro.TH`,
		"microlens-mtl":                     `Lens.Micro.Mtl`,
		"QuickCheck":                        `Test.QuickCheck`,
		"quickcheck-instances":              `Test.QuickCheck.Instances`,
		"hspec":                             `Test.Hspec`,
		"hspec-expectations":                `Test.Hspec.Expectations`,
		"hspec-wai":                         `Test.Hspec.Wai`,
		"tasty":                             `Test.Tasty`,
		"tasty-hunit":                       `Test.Tasty.HUnit`,
		"tasty-quickcheck":                  `Test.Tasty.QuickCheck`,
		"tasty-golden":                      `Test.Tasty.Golden`,
		"tasty-hspec":                       `Test.Tasty.Hspec`,
		"tasty-smallcheck":                  `Test.Tasty.SmallCheck`,
		"tasty-hedgehog":                    `Test.Tasty.Hedgehog`,
		"tasty-bench":                       `Test.Tasty.Bench`,
		"HUnit":                             `Test.HUnit`,
		"smallcheck":                        `Test.SmallCheck`,
		"hedgehog":                          `Hedgehog`,
		"doctest doctest-parallel":          `Test.DocTest`,
		"criterion":                         `Criterion`,
		"servant":                           `Servant.API`,
		"servant-server servant":            `Servant`,
		"servant-server":                    `Servant.Server`,
		"servant-client":                    `Servant.Client`,
		"warp":                              `Network.Wai.Handler.Warp`,
		"warp-tls":                          `Network.Wai.Handler.WarpTLS`,
		"wai":                               `Network.Wai`,
		"wai-extra":                         `Network.Wai.Middleware Network.Wai.Test Network.Wai.Parse Network.Wai.Handler.CGI`,
		"http-client":                       `Network.HTTP.Client`,
		"http-client-tls":                   `Network.HTTP.Client.TLS`,
		"http-types":                        `Network.HTTP.Types`,
		"http-conduit":                      `Network.HTTP.Simple Network.HTTP.Conduit Network.HTTP.Client.Conduit`,
		"req":                               `Network.HTTP.Req`,
		"network":                           `Network.Socket Network.BSD`,
		"network-uri":                       `Network.URI`,
		"websockets":                        `Network.WebSockets`,
		"wreq":                              `Network.Wreq`,
		"tls":                               `Network.TLS`,
		"mime-types":                        `Network.Mime`,
		"async":                             `Control.Concurrent.Async`,
		"stm":                               `Control.Concurrent.STM Control.Monad.STM`,
		"time":                              `Data.Time`,
		"directory":                         `System.Directory`,
		"filepath":                          `System.FilePath System.OsPath`,
		"process":                           `System.Process System.Cmd`,
		"deepseq":                           `Control.DeepSeq`,
		"random":                            `System.Random`,
		"parsec":                            `Text.Parsec Text.ParserCombinators.Parsec`,
		"megaparsec":                        `Text.Megaparsec`,
		"parser-combinators":                `Control.Applicative.Combinators Control.Monad.Combinators Control.Applicative.Permutations`,
		"optparse-applicative":              `Options.Applicative`,
		"conduit":                           `Data.Conduit Conduit`,
		"conduit-extra":                     `Data.Conduit.Binary Data.Conduit.Process Data.Conduit.Network Data.Conduit.Attoparsec`,
		"attoparsec":                        `Data.Attoparsec`,
		"scientific":                        `Data.Scientific Data.ByteString.Builder.Scientific Data.Text.Lazy.Builder.Scientific`,
		"array":                             `Data.Array`,
		"unix":                              `System.Posix`,
		"Win32":                             `System.Win32 Graphics.Win32`,
		"exceptions":                        `Control.Monad.Catch`,
		"safe-exceptions":                   `Control.Exception.Safe`,
		"unliftio":                          `UnliftIO`,
		"unliftio-core":                     `Control.Monad.IO.Unlift`,
		"resourcet":                         `Control.Monad.Trans.Resource Data.Acquire`,
		"primitive":                         `Control.Monad.Primitive Data.Primitive`,
		"data-default":                      `Data.Default`,
		"data-default-class":                `Data.Default.Class`,
		"case-insensitive":                  `Data.CaseInsensitive`,
		"base64-bytestring":                 `Data.ByteString.Base64`,
		"base16-bytestring":                 `Data.ByteString.Base16`,
		"utf8-string":                       `Data.ByteString.UTF8 Data.ByteString.Lazy.UTF8 Codec.Binary.UTF8`,
		"binary":                            `Data.Binary`,
		"cereal":                            `Data.Serialize`,
		"zlib":                              `Codec.Compression.Zlib Codec.Compression.GZip Codec.Compression`,
		"tar":                               `Codec.Archive.Tar`,
		"zip-archive":                       `Codec.Archive.Zip`,
		"yaml":                              `Data.Yaml`,
		"HsYAML":                            `Data.YAML`,
		"xml-conduit":                       `Text.XML`,
		"xml":                               `Text.XML.Light`,
		"blaze-html":                        `Text.Blaze`,
		"blaze-markup":                      `Text.Blaze.Internal Text.Blaze.Renderer`,
		"lucid":                             `Lucid`,
		"doctemplates":                      `Text.DocTemplates`,
		"doclayout":                         `Text.DocLayout`,
		"pandoc-types":                      `Text.Pandoc.Definition Text.Pandoc.Builder Text.Pandoc.Generic Text.Pandoc.Walk Text.Pandoc.JSON Text.Pandoc.Arbitrary`,
		"skylighting":                       `Skylighting`,
		"texmath":                           `Text.TeXMath`,
		"citeproc":                          `Citeproc`,
		"commonmark":                        `Commonmark`,
		"commonmark-extensions":             `Commonmark.Extensions`,
		"commonmark-pandoc":                 `Commonmark.Pandoc`,
		"persistent":                        `Database.Persist`,
		"persistent-sqlite":                 `Database.Persist.Sqlite Database.Sqlite`,
		"persistent-postgresql":             `Database.Persist.Postgresql`,
		"esqueleto":                         `Database.Esqueleto`,
		"postgresql-simple":                 `Database.PostgreSQL.Simple`,
		"postgresql-libpq":                  `Database.PostgreSQL.LibPQ`,
		"sqlite-simple":                     `Database.SQLite.Simple`,
		"split":                             `Data.List.Split`,
		"extra":                             `Extra Data.List.Extra Control.Monad.Extra Data.Tuple.Extra System.IO.Extra System.Process.Extra Control.Exception.Extra Data.Either.Extra Data.IORef.Extra System.Directory.Extra System.Info.Extra System.Time.Extra Numeric.Extra Data.Typeable.Extra Control.Concurrent.Extra Data.Version.Extra Text.Read.Extra`,
		"safe":                              `Safe`,
		"tagged":                            `Data.Tagged`,
		"profunctors":                       `Data.Profunctor`,
		"comonad":                           `Control.Comonad`,
		"free":                              `Control.Monad.Free Control.Monad.Trans.Free Control.Alternative.Free Control.Applicative.Free`,
		"generic-lens":                      `Data.Generics.Product Data.Generics.Sum Data.Generics.Labels Data.Generics.Wrapped`,
		"syb":                               `Data.Generics`,
		"uuid":                              `Data.UUID`,
		"uuid-types":                        `Data.UUID.Types`,
		"crypton cryptonite":                `Crypto.Hash Crypto.Cipher Crypto.Random Crypto.PubKey Crypto.MAC Crypto.KDF Crypto.Error Crypto.Number Crypto.Data`,
		"memory ram":                        `Data.ByteArray`,
		"jose":                              `Crypto.JOSE Crypto.JWT`,
		"monad-logger":                      `Control.Monad.Logger`,
		"fast-logger":                       `System.Log.FastLogger`,
		"katip":                             `Katip`,
		"rio":                               `RIO`,
		"relude":                            `Relude`,
		"protolude":                         `Protolude`,
		"pretty":                            `Text.PrettyPrint`,
		"prettyprinter":                     `Prettyprinter Data.Text.Prettyprint.Doc`,
		"ansi-terminal":                     `System.Console.ANSI`,
		"ansi-wl-pprint":                    `Text.PrettyPrint.ANSI.Leijen`,
		"haskeline":                         `System.Console.Haskeline`,
		"th-abstraction":                    `Language.Haskell.TH.Datatype`,
		"th-lift":                           `Language.Haskell.TH.Lift`,
		"th-compat":                         `Language.Haskell.TH.Syntax.Compat`,
		"dlist":                             `Data.DList`,
		"semialign":                         `Data.Semialign Data.Align Data.Zip`,
		"these":                             `Data.These`,
		"witherable":                        `Witherable Data.Witherable`,
		"indexed-traversable":               `Data.Foldable.WithIndex Data.Functor.WithIndex Data.Traversable.WithIndex`,
		"integer-logarithms":                `Math.NumberTheory.Logarithms`,
		"time-compat":                       `Data.Time.Compat Data.Time.Calendar.Compat Data.Time.Clock.Compat Data.Time.LocalTime.Compat Data.Time.Format.Compat`,
		"strict":                            `Data.Strict`,
		"text-short":                        `Data.Text.Short`,
		"file-embed":                        `Data.FileEmbed`,
		"Cabal":                             `Distribution`,
		"Cabal-syntax":                      `Distribution.Parsec Distribution.Pretty Distribution.Fields Distribution.Types Distribution.PackageDescription Distribution.Version Distribution.License`,
		"hpack":                             `Hpack`,
		"path":                              `Path`,
		"path-io":                           `Path.IO`,
		"pantry":                            `Pantry`,
		"fsnotify":                          `System.FSNotify`,
		"temporary":                         `System.IO.Temp`,
		"filelock":                          `System.FileLock`,
		"hslua":                             `HsLua`,
		"lua":                               `Lua Foreign.Lua`,
		"hasql":                             `Hasql`,
		"regex-tdfa":                        `Text.Regex.TDFA`,
		"regex-base":                        `Text.Regex.Base`,
		"regex-posix":                       `Text.Regex.Posix`,
		"regex-pcre":                        `Text.Regex.PCRE`,
		"mustache":                          `Text.Mustache`,
		"neat-interpolation":                `NeatInterpolation`,
		"string-interpolate":                `Data.String.Interpolate`,
		"bifunctors":                        `Data.Bifunctor.Biff Data.Bifunctor.Clown Data.Bifunctor.Flip Data.Bifunctor.Join Data.Bifunctor.Joker Data.Bifunctor.Tannen Data.Bifunctor.Wrapped Data.Bifunctor.TH`,
		"contravariant":                     `Data.Functor.Contravariant.Divisible Data.Functor.Contravariant.Generic`,
		"recursion-schemes":                 `Data.Functor.Foldable`,
		"kan-extensions":                    `Data.Functor.Yoneda Data.Functor.Coyoneda Data.Functor.Day Control.Monad.Codensity`,
		"semigroupoids":                     `Data.Functor.Apply Data.Functor.Bind Data.Functor.Alt Data.Functor.Plus Data.Semigroupoid Data.Semigroup.Foldable Data.Semigroup.Traversable`,
		"base-compat base-compat-batteries": `Prelude.Compat Data.List.Compat Control.Monad.Compat Data.Functor.Compat`,
		"splitmix":                          `System.Random.SplitMix`,
		"generic-deriving":                  `Generics.Deriving`,
		"streaming-commons":                 `Data.Streaming`,
		"typed-process":                     `System.Process.Typed`,
		"cryptohash-sha256":                 `Crypto.Hash.SHA256`,
		"SHA":                               `Data.Digest.Pure.SHA`,
		"digest":                            `Data.Digest.CRC32 Data.Digest.Adler32`,
		"blaze-builder":                     `Blaze.ByteString.Builder`,
		"vault":                             `Data.Vault`,
		"cookie":                            `Web.Cookie`,
		"jwt":                               `Web.JWT`,
		"http2":                             `Network.HTTP2`,
		"iproute":                           `Data.IP`,
		"auto-update":                       `Control.AutoUpdate Control.Debounce Control.Reaper`,
		"old-locale":                        `System.Locale`,
		"old-time":                          `System.Time`,
		"monad-control":                     `Control.Monad.Trans.Control`,
		"transformers-base":                 `Control.Monad.Base`,
		"lifted-base":                       `Control.Exception.Lifted Control.Concurrent.Lifted Control.Concurrent.MVar.Lifted`,
		"lifted-async":                      `Control.Concurrent.Async.Lifted`,
		"stm-chans":                         `Control.Concurrent.STM.TBMQueue Control.Concurrent.STM.TMQueue Control.Concurrent.STM.TBMChan`,
		"quickcheck-io":                     `Test.QuickCheck.IO`,
		"hspec-core":                        `Test.Hspec.Core`,
		"hspec-discover":                    `Test.Hspec.Discover`,
		"hspec-golden":                      `Test.Hspec.Golden`,
		"tasty-expected-failure":            `Test.Tasty.ExpectedFailure`,
		"Diff":                              `Data.Algorithm.Diff`,
		"crypton-connection connection":     `Network.Connection`,
		"haddock-library":                   `Documentation.Haddock`,
		"gridtables":                        `Text.GridTable`,
		"jira-wiki-markup":                  `Text.Jira`,
		"unicode-data":                      `Unicode.Char`,
		"crypton-x509-system x509-system":   `System.X509`,
		"crypton-x509 x509":                 `Data.X509`,
		"optparse-generic":                  `Options.Generic`,
		"raw-strings-qq":                    `Text.RawString.QQ`,
		"semaphore-compat":                  `System.Semaphore`,
		"Ranged-sets":                       `Data.Ranged`,
		"swagger2":                          `Data.Swagger`,
		"character-ps":                      `Data.Word8.Patterns Data.Word16.Patterns`,
		"ghc-exactprint":                    `Language.Haskell.GHC.ExactPrint`,
		"openapi3":                          `Data.OpenApi`,
		"sop-core":                          `Data.SOP`,
		"generics-sop":                      `Generics.SOP`,
		"mmorph":                            `Control.Monad.Morph`,
		"transformers-compat":               `Control.Monad.Trans.Instances`,
		"network-bsd":                       `Network.BSD`,
		"unix-compat":                       `System.PosixCompat`,
		"zlib-bindings":                     `Codec.Zlib`,
		"JuicyPixels":                       `Codec.Picture`,
		"xml-types":                         `Data.XML.Types`,
		"html-conduit":                      `Text.HTML.DOM`,
		"tagsoup":                           `Text.HTML.TagSoup Text.StringLike`,
		"emojis":                            `Text.Emoji`,
		"unicode-collation":                 `Text.Collate`,
		"unicode-transforms":                `Data.Text.Normalize Data.ByteString.UTF8.Normalize`,
		"data-fix":                          `Data.Fix`,
		"OneTuple":                          `Data.Tuple.Solo`,
		"hashtables":                        `Data.HashTable`,
		"parallel":                          `Control.Parallel`,
		"clock":                             `System.Clock`,
		"hourglass":                         `Data.Hourglass`,
		"text-builder":                      `TextBuilder`,
		"aeson-qq":                          `Data.Aeson.QQ`,
		"insert-ordered-containers":         `Data.HashMap.Strict.InsOrd`,
		"configurator-pg":                   `Data.Configurator`,
		"jose-jwt":                          `Jose`,
		"wai-cors":                          `Network.Wai.Middleware.Cors`,
		"wai-logger":                        `Network.Wai.Logger`,
		"prometheus-client":                 `Prometheus`,
		"hasql-pool":                        `Hasql.Pool`,
		"hasql-transaction":                 `Hasql.Transaction`,
		"hasql-notifications":               `Hasql.Notifications`,
		"hasql-dynamic-statements":          `Hasql.DynamicStatements`,
		"heredoc":                           `Text.Heredoc`,
		"either":                            `Data.Either.Combinators Data.Either.Validation`,
		"errors":                            `Control.Error`,
		"interpolatedstring-perl6":          `Text.InterpolatedString.Perl6`,
		"gitrev":                            `Development.GitRev`,
		"githash":                           `GitHash`,
		"shake":                             `Development.Shake`,
		"directory-ospath-streaming":        `System.Directory.OsPath.Streaming`,
		"os-string":                         `System.OsString`,
		"network-info":                      `Network.Info`,
		"retry":                             `Control.Retry`,
		"cryptohash":                        `Crypto.Hash.MD5 Crypto.Hash.SHA1`,
		"base-orphans":                      `Data.Orphans`,
		"assoc":                             `Data.Bifunctor.Assoc Data.Bifunctor.Swap`,
		"distributive":                      `Data.Distributive`,
		"void":                              `Data.Void.Unsafe`,
		"colour":                            `Data.Colour`,
		"linear":                            `Linear`,
		"statistics":                        `Statistics`,
		"math-functions":                    `Numeric.SpecFunctions Numeric.Sum Numeric.MathFunctions`,
		"mwc-random":                        `System.Random.MWC`,
		"gloss":                             `Graphics.Gloss`,
		"brick":                             `Brick`,
		"vty":                               `Graphics.Vty`,
		"scotty":                            `Web.Scotty`,
		"yesod":                             `Yesod`,
		"yesod-core":                        `Yesod.Core`,
		"ihp":                               `IHP`,
		"polysemy":                          `Polysemy`,
		"effectful":                         `Effectful`,
		"fused-effects":                     `Control.Effect Control.Carrier Control.Algebra`,
		"freer-simple":                      `Control.Monad.Freer`,
		"amazonka":                          `Amazonka Network.AWS`,
		"stm-containers":                    `StmContainers`,
		"focus":                             `Focus`,
		"list-t":                            `ListT`,
		"record-dot-preprocessor":           `RecordDotPreprocessor`,
		"optics":                            `Optics`,
		"optics-core":                       `Optics.Core`,
		"network-simple":                    `Network.Simple.TCP`,
		"dhall":                             `Dhall`,
		"toml-parser":                       `Toml`,
		"tomland":                           `Toml.Codec`,
		"cassava":                           `Data.Csv`,
		"bytestring-lexing":                 `Data.ByteString.Lex`,
		"text-icu":                          `Data.Text.ICU`,
		"template":                          `Data.Text.Template`,
		"monad-par":                         `Control.Monad.Par`,
		"beam-core":                         `Database.Beam`,
		"opaleye":                           `Opaleye`,
		"selda":                             `Database.Selda`,
		"mysql-simple":                      `Database.MySQL.Simple`,
		"hedis":                             `Database.Redis`,
		"mongoDB":                           `Database.MongoDB`,
		"groundhog":                         `Database.Groundhog`,
		"HDBC":                              `Database.HDBC`,
		"haskell-src-exts":                  `Language.Haskell.Exts`,
		"haskell-src-meta":                  `Language.Haskell.Meta`,
		"hie-bios":                          `HIE.Bios`,
		"lsp":                               `Language.LSP.Server`,
		"lsp-types":                         `Language.LSP.Protocol`,
		"text-rope":                         `Data.Text.Rope`,
		"hi-file-parser":                    `HiFileParser`,
		"http-download":                     `Network.HTTP.Download`,
		"casa-client":                       `Casa.Client`,
		"casa-types":                        `Casa.Types`,
		"project-template":                  `Text.ProjectTemplate`,
		"open-browser":                      `Web.Browser`,
		"echo":                              `System.IO.Echo`,
		"companion":                         `Control.Concurrent.Companion`,
		"tar-conduit":                       `Data.Conduit.Tar`,
		"rio-prettyprint":                   `RIO.PrettyPrint`,
		"aeson-warning-parser":              `Data.Aeson.WarningParser`,
		"persistent-template":               `Database.Persist.TH`,
		"bytestring-conversion":             `Data.ByteString.Conversion`,
		"fuzzyset":                          `Data.FuzzySet`,
		"aeson-jsonpath":                    `Data.Aeson.JSONPath`,
		"postgresql-binary":                 `PostgreSQL.Binary`,
		"cache":                             `Data.Cache`,
		"timeit":                            `System.TimeIt`,
		"optparse-simple":                   `Options.Applicative.Simple`,
		"cmdargs":                           `System.Console.CmdArgs`,
	} {
		for _, m := range strings.Fields(mods) {
			moduleTable[m] = append(moduleTable[m], strings.Fields(pkgs)...)
		}
	}
}

// tableMatch is the longest key of moduleTable that is mod or a prefix of it (on a
// dot), and its packages; allowed, when set, limits the packages considered.
func tableMatch(mod string, allowed func(string) bool) (string, []string) {
	for m := mod; ; {
		if pkgs := moduleTable[m]; len(pkgs) > 0 {
			var ok []string
			for _, p := range pkgs {
				if allowed == nil || allowed(p) {
					ok = append(ok, p)
				}
			}
			if len(ok) > 0 {
				return m, ok
			}
		}
		i := strings.LastIndexByte(m, '.')
		if i < 0 {
			return "", nil
		}
		m = m[:i]
	}
}

// categories are the first segments of the module hierarchy that name a subject,
// not a package (Data.Default is data-default's, Network.Wai wai's, Test.QuickCheck
// QuickCheck's).
var categories = map[string]bool{
	"Data": true, "Control": true, "Text": true, "System": true, "Network": true, "Test": true,
	"Database": true, "Codec": true, "Web": true, "Graphics": true, "Language": true,
	"Development": true, "Distribution": true, "Foreign": true, "Numeric": true, "Math": true,
	"Algebra": true, "Sound": true, "Generics": true, "Unsafe": true, "Debug": true, "Monad": true,
}

// fold is a package or module-run name compared loosely: lower case, without dashes,
// underscores and dots (Hasql.Pool is hasql-pool, Database.PostgreSQL.Simple
// postgresql-simple).
func fold(s string) string {
	s = strings.ToLower(s)
	return strings.NewReplacer("-", "", "_", "", ".", "").Replace(s)
}

// runMatch looks for a package among names whose name is a run of the module's
// segments: Test.Tasty.HUnit is tasty-hunit (or tasty), Network.HTTP.Client
// http-client. The longest run wins; a run of only a first segment that is a subject
// (Text, Data) does not count.
func runMatch(mod string, names map[string]string) string {
	segs := strings.Split(mod, ".")
	for n := len(segs); n >= 1; n-- {
		for i := 0; i+n <= len(segs); i++ {
			if n == 1 && categories[segs[i]] {
				continue
			}
			if p := names[fold(strings.Join(segs[i:i+n], ""))]; p != "" {
				return p
			}
		}
	}
	return ""
}

// guessName names the package of a module nothing else resolved: its first segment
// that does not name a subject, lower case (Network.Wai: wai, Hasql.Pool: hasql).
func guessName(mod string) string {
	segs := strings.Split(mod, ".")
	for i, s := range segs {
		if !categories[s] || i == len(segs)-1 {
			return strings.ToLower(s)
		}
	}
	return strings.ToLower(segs[0])
}
