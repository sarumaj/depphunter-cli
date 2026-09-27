// One file, several classes, and not in a directory named after its package.
package com.example.app.model

final case class User(name: String)

case class Order(id: Long, user: User)
