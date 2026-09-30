package swift

import (
	"strings"
	"sync"
)

// stdModules are the modules every Swift toolchain ships on every platform: the
// standard library and its satellites, the core libraries (Foundation, Dispatch,
// XCTest), swift-testing, the manifest APIs, and the C libraries of the platforms
// Swift runs on besides Apple's (Glibc, Musl, WinSDK...).
//
// Implements: REQ-SWIFT-005
var stdModules = set(
	"Swift", "SwiftOnoneSupport", "_Concurrency", "_StringProcessing", "_Differentiation",
	"_Volatile", "RegexBuilder", "Observation", "Distributed", "Synchronization",
	"Cxx", "CxxStdlib", "Foundation", "FoundationEssentials",
	"FoundationInternationalization", "FoundationNetworking", "FoundationXML",
	"Dispatch", "XCTest", "Testing", "PackageDescription", "PackagePlugin",
	"CompilerPluginSupport", "Glibc", "Musl", "WinSDK", "ucrt", "CRT", "Android",
	"Bionic", "WASILibc", "SwiftGlibc", "Builtin",
)

// appleModules are the frameworks of Apple's SDKs (iOS, macOS, tvOS, watchOS,
// visionOS) and the Darwin C modules: they come with Xcode, not from a package.
//
// Implements: REQ-SWIFT-005
var appleModules = set(
	"Darwin", "ObjectiveC", "os", "MachO", "CoreFoundation", "Security", "System",
	"UIKit", "AppKit", "SwiftUI", "Combine", "WatchKit", "TVUIKit", "WidgetKit",
	"CoreData", "SwiftData", "CoreGraphics", "CoreImage", "CoreText", "CoreVideo",
	"CoreMedia", "CoreAudio", "CoreAudioKit", "CoreAudioTypes", "CoreMIDI",
	"CoreLocation", "CoreLocationUI", "CoreBluetooth", "CoreMotion", "CoreML",
	"CoreNFC", "CoreSpotlight", "CoreServices", "CoreTelephony", "CoreHaptics",
	"CoreTransferable", "CoreWLAN", "CryptoKit", "CommonCrypto", "LocalAuthentication",
	"AuthenticationServices", "AVFoundation", "AVKit", "AVFAudio", "AVRouting",
	"AudioToolbox", "AudioUnit", "MediaPlayer", "MediaToolbox", "Photos", "PhotosUI",
	"PhotoKit", "PDFKit", "QuickLook", "QuickLookThumbnailing", "QuartzCore", "Quartz",
	"Metal", "MetalKit", "MetalPerformanceShaders", "MetalPerformanceShadersGraph",
	"MetalFX", "SceneKit", "SpriteKit", "GameKit", "GameController", "GameplayKit",
	"ARKit", "RealityKit", "RealityFoundation", "Vision", "VisionKit", "NaturalLanguage",
	"Speech", "SoundAnalysis", "CreateML", "MapKit", "Contacts", "ContactsUI",
	"EventKit", "EventKitUI", "HealthKit", "HealthKitUI", "HomeKit", "StoreKit",
	"MessageUI", "Messages", "SafariServices", "WebKit", "JavaScriptCore",
	"UserNotifications", "UserNotificationsUI", "Network", "NetworkExtension",
	"SystemConfiguration", "CFNetwork", "MultipeerConnectivity", "BackgroundTasks",
	"CloudKit", "PassKit", "PushKit", "CallKit", "Intents", "IntentsUI", "AppIntents",
	"ActivityKit", "AppTrackingTransparency", "AdSupport", "AdServices", "iAd",
	"Accessibility", "Accelerate", "simd", "GLKit", "OpenGLES", "OpenGL",
	"ImageIO", "UniformTypeIdentifiers", "MobileCoreServices", "LinkPresentation",
	"FileProvider", "FileProviderUI", "DeviceCheck", "ExternalAccessory", "IOKit",
	"IOSurface", "IOBluetooth", "Cocoa", "Carbon", "ApplicationServices",
	"ServiceManagement", "OSLog", "Social", "Accounts", "SharedWithYou", "GroupActivities",
	"ShazamKit", "WeatherKit", "Charts", "TipKit", "Translation", "MusicKit",
	"ClockKit", "ScreenCaptureKit", "ScreenTime", "FamilyControls", "ManagedSettings",
	"DeviceActivity", "BrowserEngineKit", "Foundation_Private", "Dispatch_Private",
	"XCUIAutomation", "StoreKitTest", "Virtualization", "Hypervisor", "DriverKit",
	"CryptoTokenKit", "OpenDirectory", "Collaboration", "ExtensionFoundation",
	"ExtensionKit", "Symbols", "PencilKit", "ClassKit", "CarPlay", "SensorKit",
	"NearbyInteraction", "HomeKitUI", "FinderSync", "QuickLookUI", "Quartz", "InputMethodKit",
	"ImageCaptureCore", "AppleScriptObjC", "OSAKit", "SecurityInterface", "SafariServices", "_AppIntents_SwiftUI", "_MapKit_SwiftUI",
	"_AVKit_SwiftUI", "_StoreKit_SwiftUI", "_PhotosUI_SwiftUI", "_SwiftData_SwiftUI",
	// Older frameworks Objective-C code still imports.
	"AddressBook", "AddressBookUI", "AssetsLibrary", "Twitter", "NotificationCenter",
	"WatchConnectivity", "OpenAL", "MediaAccessibility", "NewsstandKit", "GSS",
	"ExceptionHandling", "PreferencePanes", "ScriptingBridge", "InstantMessage", "DiscRecording",
	"LocalAuthenticationEmbeddedUI", "DeviceDiscoveryUI", "PlaygroundSupport",
	"notify", "zlib", "Compression", "XPC",
)

// AppleEcosystem is the island of Apple's SDK frameworks, which the objc plugin
// shares.
const AppleEcosystem = ecosystemApple

// AppleSDK names the Apple SDK framework a module or framework directory is, for
// the objc plugin, in its own spelling: case is ignored, since Xcode's default file
// system is case-insensitive and <Appkit/Appkit.h> compiles. Foundation and XCTest
// count too, which Swift counts as its toolchain's (swift-corelibs-foundation,
// swift-corelibs-xctest) but an Objective-C program gets from Xcode.
//
// Implements: REQ-SWIFT-005, REQ-OBJC-005
func AppleSDK(name string) (string, bool) {
	if appleModules[name] {
		return name, true
	}
	n, ok := appleFolded()[strings.ToLower(name)]
	return n, ok
}

var appleFolded = sync.OnceValue(func() map[string]string {
	m := map[string]string{"foundation": "Foundation", "xctest": "XCTest", "dispatch": "Dispatch"}
	for n := range appleModules {
		m[strings.ToLower(n)] = n
	}
	return m
})

// knownPackages names the package of modules whose names say little about it:
// module -> repository URL (without scheme), from which the SwiftPM identity is the
// last segment. A module of a declared package is matched by name (NIOCore is
// swift-nio's, Collections swift-collections') without this; the table is for
// packages whose modules are named otherwise (Logging is swift-log's) and names the
// package of an undeclared one.
//
// Implements: REQ-SWIFT-007
var knownPackages = map[string]string{
	"NIO": "github.com/apple/swift-nio", "NIOCore": "github.com/apple/swift-nio",
	"NIOPosix": "github.com/apple/swift-nio", "NIOHTTP1": "github.com/apple/swift-nio",
	"NIOWebSocket": "github.com/apple/swift-nio", "NIOEmbedded": "github.com/apple/swift-nio",
	"NIOConcurrencyHelpers": "github.com/apple/swift-nio", "NIOFoundationCompat": "github.com/apple/swift-nio",
	"NIOTLS": "github.com/apple/swift-nio", "_NIOConcurrency": "github.com/apple/swift-nio",
	"NIOTestUtils": "github.com/apple/swift-nio", "NIOFileSystem": "github.com/apple/swift-nio",
	"_NIOFileSystem": "github.com/apple/swift-nio",
	"NIOHTTP2":       "github.com/apple/swift-nio-http2", "NIOSSL": "github.com/apple/swift-nio-ssl",
	"NIOExtras": "github.com/apple/swift-nio-extras", "NIOHTTPCompression": "github.com/apple/swift-nio-extras",
	"NIOTransportServices": "github.com/apple/swift-nio-transport-services",
	"Logging":              "github.com/apple/swift-log", "Metrics": "github.com/apple/swift-metrics",
	"CoreMetrics":    "github.com/apple/swift-metrics",
	"ArgumentParser": "github.com/apple/swift-argument-parser",
	"Collections":    "github.com/apple/swift-collections", "DequeModule": "github.com/apple/swift-collections",
	"OrderedCollections": "github.com/apple/swift-collections", "HeapModule": "github.com/apple/swift-collections",
	"Algorithms": "github.com/apple/swift-algorithms",
	"Numerics":   "github.com/apple/swift-numerics", "RealModule": "github.com/apple/swift-numerics",
	"Crypto": "github.com/apple/swift-crypto", "_CryptoExtras": "github.com/apple/swift-crypto",
	"AsyncAlgorithms": "github.com/apple/swift-async-algorithms",
	"Atomics":         "github.com/apple/swift-atomics",
	"SystemPackage":   "github.com/apple/swift-system",
	"SwiftProtobuf":   "github.com/apple/swift-protobuf",
	"Tracing":         "github.com/apple/swift-distributed-tracing", "Instrumentation": "github.com/apple/swift-distributed-tracing",
	"ServiceContextModule": "github.com/apple/swift-service-context",
	"AsyncHTTPClient":      "github.com/swift-server/async-http-client",
	"ServiceLifecycle":     "github.com/swift-server/swift-service-lifecycle",
	"SwiftSyntax":          "github.com/swiftlang/swift-syntax", "SwiftSyntaxMacros": "github.com/swiftlang/swift-syntax",
	"SwiftSyntaxBuilder": "github.com/swiftlang/swift-syntax", "SwiftCompilerPlugin": "github.com/swiftlang/swift-syntax",
	"SwiftParser": "github.com/swiftlang/swift-syntax", "SwiftDiagnostics": "github.com/swiftlang/swift-syntax",
	"SwiftSyntaxMacrosTestSupport": "github.com/swiftlang/swift-syntax",
	"Markdown":                     "github.com/swiftlang/swift-markdown",
	"Vapor":                        "github.com/vapor/vapor", "XCTVapor": "github.com/vapor/vapor",
	"Fluent": "github.com/vapor/fluent", "FluentKit": "github.com/vapor/fluent-kit",
	"Leaf": "github.com/vapor/leaf", "JWT": "github.com/vapor/jwt", "JWTKit": "github.com/vapor/jwt-kit",
	"Alamofire": "github.com/Alamofire/Alamofire",
	"RxSwift":   "github.com/ReactiveX/RxSwift", "RxCocoa": "github.com/ReactiveX/RxSwift",
	"RxRelay": "github.com/ReactiveX/RxSwift", "RxTest": "github.com/ReactiveX/RxSwift",
	"RxBlocking": "github.com/ReactiveX/RxSwift",
	"SnapKit":    "github.com/SnapKit/SnapKit",
	"Kingfisher": "github.com/onevcat/Kingfisher",
	"SwiftyJSON": "github.com/SwiftyJSON/SwiftyJSON",
	"Moya":       "github.com/Moya/Moya",
	"Realm":      "github.com/realm/realm-swift", "RealmSwift": "github.com/realm/realm-swift",
	"GRDB":   "github.com/groue/GRDB.swift",
	"SQLite": "github.com/stephencelis/SQLite.swift",
	"Quick":  "github.com/Quick/Quick", "Nimble": "github.com/Quick/Nimble",
	"SnapshotTesting":          "github.com/pointfreeco/swift-snapshot-testing",
	"ComposableArchitecture":   "github.com/pointfreeco/swift-composable-architecture",
	"Dependencies":             "github.com/pointfreeco/swift-dependencies",
	"CustomDump":               "github.com/pointfreeco/swift-custom-dump",
	"CasePaths":                "github.com/pointfreeco/swift-case-paths",
	"IdentifiedCollections":    "github.com/pointfreeco/swift-identified-collections",
	"KeychainAccess":           "github.com/kishikawakatsumi/KeychainAccess",
	"Lottie":                   "github.com/airbnb/lottie-ios",
	"SDWebImage":               "github.com/SDWebImage/SDWebImage",
	"Swinject":                 "github.com/Swinject/Swinject",
	"PromiseKit":               "github.com/mxcl/PromiseKit",
	"Yams":                     "github.com/jpsim/Yams",
	"Rainbow":                  "github.com/onevcat/Rainbow",
	"Files":                    "github.com/JohnSundell/Files",
	"Sparkle":                  "github.com/sparkle-project/Sparkle",
	"Starscream":               "github.com/daltoniam/Starscream",
	"Kitura":                   "github.com/Kitura/Kitura",
	"Hummingbird":              "github.com/hummingbird-project/hummingbird",
	"GRPC":                     "github.com/grpc/grpc-swift",
	"FirebaseCore":             "github.com/firebase/firebase-ios-sdk",
	"FirebaseAnalytics":        "github.com/firebase/firebase-ios-sdk",
	"FirebaseAuth":             "github.com/firebase/firebase-ios-sdk",
	"FirebaseFirestore":        "github.com/firebase/firebase-ios-sdk",
	"FirebaseMessaging":        "github.com/firebase/firebase-ios-sdk",
	"FirebaseCrashlytics":      "github.com/firebase/firebase-ios-sdk",
	"FirebaseStorage":          "github.com/firebase/firebase-ios-sdk",
	"FirebaseDatabase":         "github.com/firebase/firebase-ios-sdk",
	"FirebaseRemoteConfig":     "github.com/firebase/firebase-ios-sdk",
	"GoogleSignIn":             "github.com/google/GoogleSignIn-iOS",
	"Sentry":                   "github.com/getsentry/sentry-cocoa",
	"SwiftLintFramework":       "github.com/realm/SwiftLint",
	"Introspect":               "github.com/siteline/SwiftUI-Introspect",
	"SwiftUIIntrospect":        "github.com/siteline/SwiftUI-Introspect",
	"Factory":                  "github.com/hmlongco/Factory",
	"Defaults":                 "github.com/sindresorhus/Defaults",
	"KeyboardShortcuts":        "github.com/sindresorhus/KeyboardShortcuts",
	"LaunchAtLogin":            "github.com/sindresorhus/LaunchAtLogin-Modern",
	"Down":                     "github.com/johnxnguyen/Down",
	"Highlightr":               "github.com/raspu/Highlightr",
	"MarkdownUI":               "github.com/gonzalezreal/swift-markdown-ui",
	"Nuke":                     "github.com/kean/Nuke",
	"NukeUI":                   "github.com/kean/Nuke",
	"Pulse":                    "github.com/kean/Pulse",
	"Get":                      "github.com/kean/Get",
	"SwiftSoup":                "github.com/scinfu/SwiftSoup",
	"ZIPFoundation":            "github.com/weichsel/ZIPFoundation",
	"CryptoSwift":              "github.com/krzyzanowskim/CryptoSwift",
	"BigInt":                   "github.com/attaswift/BigInt",
	"PathKit":                  "github.com/kylef/PathKit",
	"Commander":                "github.com/kylef/Commander",
	"Stencil":                  "github.com/stencilproject/Stencil",
	"XcodeProj":                "github.com/tuist/XcodeProj",
	"SwiftFormat":              "github.com/nicklockwood/SwiftFormat",
	"SourceKittenFramework":    "github.com/jpsim/SourceKitten",
	"OHHTTPStubs":              "github.com/AliSoftware/OHHTTPStubs",
	"OHHTTPStubsSwift":         "github.com/AliSoftware/OHHTTPStubs",
	"Mocker":                   "github.com/WeTransfer/Mocker",
	"ViewInspector":            "github.com/nalexn/ViewInspector",
	"Cuckoo":                   "github.com/Brightify/Cuckoo",
	"Then":                     "github.com/devxoul/Then",
	"Hero":                     "github.com/HeroTransitions/Hero",
	"IQKeyboardManagerSwift":   "github.com/hackiftekhar/IQKeyboardManager",
	"MBProgressHUD":            "github.com/jdg/MBProgressHUD",
	"DGCharts":                 "github.com/danielgindi/Charts",
	"CocoaLumberjack":          "github.com/CocoaLumberjack/CocoaLumberjack",
	"CocoaLumberjackSwift":     "github.com/CocoaLumberjack/CocoaLumberjack",
	"SwiftyBeaver":             "github.com/SwiftyBeaver/SwiftyBeaver",
	"Reachability":             "github.com/ashleymills/Reachability.swift",
	"ObjectMapper":             "github.com/tristanhimmelman/ObjectMapper",
	"SwiftDate":                "github.com/malcommac/SwiftDate",
	"Eureka":                   "github.com/xmartlabs/Eureka",
	"Kanna":                    "github.com/tid-kijyun/Kanna",
	"Just":                     "github.com/dduan/Just",
	"ReactiveSwift":            "github.com/ReactiveCocoa/ReactiveSwift",
	"ReactiveCocoa":            "github.com/ReactiveCocoa/ReactiveCocoa",
	"SwiftCheck":               "github.com/typelift/SwiftCheck",
	"PerfectLib":               "github.com/PerfectlySoft/Perfect",
	"MongoSwift":               "github.com/mongodb/mongo-swift-driver",
	"PostgresNIO":              "github.com/vapor/postgres-nio",
	"MySQLNIO":                 "github.com/vapor/mysql-nio",
	"RediStack":                "github.com/swift-server/RediStack",
	"SotoCore":                 "github.com/soto-project/soto-core",
	"OpenAPIRuntime":           "github.com/apple/swift-openapi-runtime",
	"OpenAPIURLSession":        "github.com/apple/swift-openapi-urlsession",
	"OpenAPIVapor":             "github.com/swift-server/swift-openapi-vapor",
	"SwiftDocC":                "github.com/swiftlang/swift-docc",
	"SwiftDocCPluginUtilities": "github.com/swiftlang/swift-docc-plugin",
	"ArgumentParserToolInfo":   "github.com/apple/swift-argument-parser",
	"TSCBasic":                 "github.com/swiftlang/swift-tools-support-core",
	"TSCUtility":               "github.com/swiftlang/swift-tools-support-core",
}

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// c99name is the module name SwiftPM gives a target: characters that cannot be in an
// identifier become "_" (target "my-lib" is module my_lib).
func c99name(name string) string {
	b := []byte(name)
	for i, c := range b {
		if !identifierCharacter(c) {
			b[i] = '_'
		}
	}
	if len(b) > 0 && b[0] >= '0' && b[0] <= '9' {
		return "_" + string(b)
	}
	return string(b)
}
