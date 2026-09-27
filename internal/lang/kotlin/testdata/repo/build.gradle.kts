import Mode.FAST

plugins {
    kotlin("jvm") version "2.0.0"
}

group = "com.example"

dependencies {
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.8.0")
    implementation("io.ktor:ktor-client-core:2.3.+")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation(libs.guava)
}

enum class Mode { FAST, SLOW }
