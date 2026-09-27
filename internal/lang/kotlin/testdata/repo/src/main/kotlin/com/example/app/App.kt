/*
 * A header comment the package scan skips.
 */
@file:JvmName("Main")

package com.example.app

import kotlinx.coroutines.launch
import kotlin.collections.List
import kotlin.math.*
import kotlin.require
import java.util.UUID
import com.example.app.model.User
import com.example.app.util.*
import com.example.app.util.shout
import com.example.legacy.Legacy
import org.acme.net.Client
import io.ktor.client.HttpClient
import okhttp3.OkHttpClient as Http
import com.google.common.collect.ImmutableList
import com.example.generated.Gen

const val VERSION = "1.0"

fun main(args: Array<String>) {
    App("demo").run()
}

class App(val name: String) {
    fun run() = println(name.shout())

    private fun helper(): Int = 1

    companion object {
        fun create(): App = App("x")
    }
}

interface Service {
    fun serve()
}

object Registry {
    fun lookup(id: String) = id
}

enum class Mode {
    FAST, SLOW;

    fun weight() = 1
}

data class Point(val x: Int, val y: Int)

typealias Name = String
