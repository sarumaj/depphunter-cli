package com.example.app.util

fun String.shout(): String = uppercase()

internal fun <T> List<T>.second(): T = this[1]
