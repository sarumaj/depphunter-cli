package java

// knownArtifacts maps Java package prefixes to the Maven artifact (group:artifact)
// that ships them, for well-known libraries whose packages the naming rules of
// artifactPrefixes do not reach: com.google.common is com.google.guava:guava,
// org.apache.commons.io is commons-io:commons-io, a Spring class sits in one of a
// dozen spring-* artifacts. The longest prefix wins. An entry names the artifact by
// its base name; a Scala library's binary suffix (_2.13, _3) is taken from the
// declaration.
//
// An entry is used three ways (see resolver.finish): it adds its prefix to the
// declared artifact it names; when only other artifacts of its group are declared,
// it names an artifact that arrives with them (jackson-annotations beside
// jackson-databind, spring-boot beside a starter); and when nothing of its group is
// declared, it names the unresolved package. Keep it to libraries common enough to
// matter, and to packages that are unambiguous.
//
// Implements: REQ-JAVA-007
var knownArtifacts = map[string]string{
	// Google
	"com.google.common":                 "com.google.guava:guava",
	"com.google.thirdparty":             "com.google.guava:guava",
	"com.google.gson":                   "com.google.code.gson:gson",
	"com.google.protobuf":               "com.google.protobuf:protobuf-java",
	"com.google.inject":                 "com.google.inject:guice",
	"com.google.errorprone.annotations": "com.google.errorprone:error_prone_annotations",
	"com.google.auto.value":             "com.google.auto.value:auto-value-annotations",

	// Apache Commons and HttpComponents
	"org.apache.commons.lang3":        "org.apache.commons:commons-lang3",
	"org.apache.commons.lang":         "commons-lang:commons-lang",
	"org.apache.commons.text":         "org.apache.commons:commons-text",
	"org.apache.commons.collections4": "org.apache.commons:commons-collections4",
	"org.apache.commons.compress":     "org.apache.commons:commons-compress",
	"org.apache.commons.math3":        "org.apache.commons:commons-math3",
	"org.apache.commons.io":           "commons-io:commons-io",
	"org.apache.commons.codec":        "commons-codec:commons-codec",
	"org.apache.commons.cli":          "commons-cli:commons-cli",
	"org.apache.commons.logging":      "commons-logging:commons-logging",
	"org.apache.commons.collections":  "commons-collections:commons-collections",
	"org.apache.http":                 "org.apache.httpcomponents:httpclient",
	"org.apache.hc.client5":           "org.apache.httpcomponents.client5:httpclient5",
	"org.apache.hc.core5":             "org.apache.httpcomponents.core5:httpcore5",
	"org.apache.logging.log4j":        "org.apache.logging.log4j:log4j-api",
	"org.apache.log4j":                "log4j:log4j",

	// Testing
	"junit":                     "junit:junit",
	"org.junit":                 "junit:junit", // JUnit 4; JUnit 5 below
	"org.junit.jupiter.api":     "org.junit.jupiter:junit-jupiter-api",
	"org.junit.jupiter.params":  "org.junit.jupiter:junit-jupiter-params",
	"org.junit.platform.suite":  "org.junit.platform:junit-platform-suite-api",
	"org.opentest4j":            "org.opentest4j:opentest4j",
	"org.junitpioneer":          "org.junit-pioneer:junit-pioneer",
	"org.hamcrest":              "org.hamcrest:hamcrest",
	"org.assertj.core":          "org.assertj:assertj-core",
	"org.mockito":               "org.mockito:mockito-core",
	"org.mockito.junit.jupiter": "org.mockito:mockito-junit-jupiter",
	"org.testng":                "org.testng:testng",
	"org.testcontainers":        "org.testcontainers:testcontainers",
	"org.openjdk.jmh":           "org.openjdk.jmh:jmh-core",

	// Jackson, logging, Lombok
	"com.fasterxml.jackson.core":       "com.fasterxml.jackson.core:jackson-core",
	"com.fasterxml.jackson.databind":   "com.fasterxml.jackson.core:jackson-databind",
	"com.fasterxml.jackson.annotation": "com.fasterxml.jackson.core:jackson-annotations",
	"org.slf4j":                        "org.slf4j:slf4j-api",
	"ch.qos.logback.classic":           "ch.qos.logback:logback-classic",
	"ch.qos.logback.core":              "ch.qos.logback:logback-core",
	"lombok":                           "org.projectlombok:lombok",

	// Java EE and Jakarta EE APIs (javax.* the JDK does not ship)
	"javax.servlet":       "javax.servlet:javax.servlet-api",
	"jakarta.servlet":     "jakarta.servlet:jakarta.servlet-api",
	"javax.persistence":   "javax.persistence:javax.persistence-api",
	"jakarta.persistence": "jakarta.persistence:jakarta.persistence-api",
	"javax.validation":    "javax.validation:validation-api",
	"jakarta.validation":  "jakarta.validation:jakarta.validation-api",
	"javax.inject":        "javax.inject:javax.inject",
	"jakarta.inject":      "jakarta.inject:jakarta.inject-api",
	"jakarta.annotation":  "jakarta.annotation:jakarta.annotation-api",
	"jakarta.xml.bind":    "jakarta.xml.bind:jakarta.xml.bind-api",
	"javax.ws.rs":         "javax.ws.rs:javax.ws.rs-api",
	"jakarta.ws.rs":       "jakarta.ws.rs:jakarta.ws.rs-api",
	"javax.cache":         "javax.cache:cache-api",

	// Spring Framework, Spring Boot, Spring Data, Spring Security
	"org.springframework.core":                    "org.springframework:spring-core",
	"org.springframework.util":                    "org.springframework:spring-core",
	"org.springframework.aot":                     "org.springframework:spring-core",
	"org.springframework.beans":                   "org.springframework:spring-beans",
	"org.springframework.context":                 "org.springframework:spring-context",
	"org.springframework.stereotype":              "org.springframework:spring-context",
	"org.springframework.cache":                   "org.springframework:spring-context",
	"org.springframework.format":                  "org.springframework:spring-context",
	"org.springframework.validation":              "org.springframework:spring-context",
	"org.springframework.scheduling":              "org.springframework:spring-context",
	"org.springframework.ui":                      "org.springframework:spring-context",
	"org.springframework.dao":                     "org.springframework:spring-tx",
	"org.springframework.transaction":             "org.springframework:spring-tx",
	"org.springframework.jdbc":                    "org.springframework:spring-jdbc",
	"org.springframework.orm":                     "org.springframework:spring-orm",
	"org.springframework.http":                    "org.springframework:spring-web",
	"org.springframework.web":                     "org.springframework:spring-web",
	"org.springframework.web.servlet":             "org.springframework:spring-webmvc",
	"org.springframework.web.reactive":            "org.springframework:spring-webflux",
	"org.springframework.test":                    "org.springframework:spring-test",
	"org.springframework.boot":                    "org.springframework.boot:spring-boot",
	"org.springframework.boot.autoconfigure":      "org.springframework.boot:spring-boot-autoconfigure",
	"org.springframework.boot.test":               "org.springframework.boot:spring-boot-test",
	"org.springframework.boot.test.autoconfigure": "org.springframework.boot:spring-boot-test-autoconfigure",
	"org.springframework.boot.actuate":            "org.springframework.boot:spring-boot-actuator",
	"org.springframework.boot.testcontainers":     "org.springframework.boot:spring-boot-testcontainers",
	"org.springframework.data":                    "org.springframework.data:spring-data-commons",
	"org.springframework.data.jpa":                "org.springframework.data:spring-data-jpa",
	"org.springframework.security":                "org.springframework.security:spring-security-core",
	"org.springframework.security.config":         "org.springframework.security:spring-security-config",
	"org.springframework.security.web":            "org.springframework.security:spring-security-web",

	// Netty: its artifacts are not named after their packages
	"io.netty.buffer":             "io.netty:netty-buffer",
	"io.netty.channel":            "io.netty:netty-transport",
	"io.netty.bootstrap":          "io.netty:netty-transport",
	"io.netty.util":               "io.netty:netty-common",
	"io.netty.resolver":           "io.netty:netty-resolver",
	"io.netty.handler":            "io.netty:netty-handler",
	"io.netty.handler.codec":      "io.netty:netty-codec",
	"io.netty.handler.codec.http": "io.netty:netty-codec-http",

	// Square
	"okhttp3":               "com.squareup.okhttp3:okhttp",
	"okhttp3.mockwebserver": "com.squareup.okhttp3:mockwebserver",
	"okhttp3.logging":       "com.squareup.okhttp3:logging-interceptor",
	"okio":                  "com.squareup.okio:okio",
	"retrofit2":             "com.squareup.retrofit2:retrofit",
	"com.squareup.moshi":    "com.squareup.moshi:moshi",

	// Kotlin libraries
	"kotlinx.coroutines":         "org.jetbrains.kotlinx:kotlinx-coroutines-core",
	"kotlinx.coroutines.test":    "org.jetbrains.kotlinx:kotlinx-coroutines-test",
	"kotlinx.serialization":      "org.jetbrains.kotlinx:kotlinx-serialization-core",
	"kotlinx.serialization.json": "org.jetbrains.kotlinx:kotlinx-serialization-json",
	"kotlinx.datetime":           "org.jetbrains.kotlinx:kotlinx-datetime",
	"org.jetbrains.annotations":  "org.jetbrains:annotations",

	// Reactive, bytecode, misc
	"reactor":                      "io.projectreactor:reactor-core",
	"io.reactivex":                 "io.reactivex.rxjava2:rxjava",
	"io.reactivex.rxjava3":         "io.reactivex.rxjava3:rxjava",
	"org.objectweb.asm":            "org.ow2.asm:asm",
	"org.yaml.snakeyaml":           "org.yaml:snakeyaml",
	"com.zaxxer.hikari":            "com.zaxxer:HikariCP",
	"io.micrometer.core":           "io.micrometer:micrometer-core",
	"com.github.benmanes.caffeine": "com.github.ben-manes.caffeine:caffeine",

	// Scala: modules split out of the Scala library, and libraries imported by name
	"scala.xml":                 "org.scala-lang.modules:scala-xml",
	"scala.util.parsing":        "org.scala-lang.modules:scala-parser-combinators",
	"scala.collection.parallel": "org.scala-lang.modules:scala-parallel-collections",
	"scala.swing":               "org.scala-lang.modules:scala-swing",
	"scala.async":               "org.scala-lang.modules:scala-async",
	"scala.scalajs":             "org.scala-js:scalajs-library",
	"scala.scalanative":         "org.scala-native:nativelib",
	"cats":                      "org.typelevel:cats-core",
	"cats.effect":               "org.typelevel:cats-effect",
	"akka.actor":                "com.typesafe.akka:akka-actor",
	"akka.actor.typed":          "com.typesafe.akka:akka-actor-typed",
	"akka.stream":               "com.typesafe.akka:akka-stream",
	"akka.http":                 "com.typesafe.akka:akka-http",
	"io.circe":                  "io.circe:circe-core",
	"io.circe.generic":          "io.circe:circe-generic",
	"io.circe.parser":           "io.circe:circe-parser",
	"zio":                       "dev.zio:zio",
	"org.scalatest":             "org.scalatest:scalatest",
	"munit":                     "org.scalameta:munit",
}
