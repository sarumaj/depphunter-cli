package com.example
package app

import scala.collection.mutable
import scala.concurrent.{Future, ExecutionContext => EC}
import scala.xml.Elem
import scala.scalajs.js
import java.time.Instant
import cats.effect._
import cats.syntax.all.*
import io.circe.given
import akka.actor.typed.{ActorSystem as AS, _}
import com.example.app.model.{User, Order}
import com.example.app.util._
import com.google.common.base.Strings
import org.acme.Tools
import org.scalatest.flatspec.AnyFlatSpec
import com.github.sbt.git.GitPlugin
import com.example.generated.Gen
import collection.immutable.ListMap
import scala.Option
import model.Order
import util.Text
import _root_.cats.effect.IO

val defaultName = "app"

def greet(name: String): String = s"hello $name"

trait Service {
  def serve(): Unit
}

class App(name: String) extends Service {
  def serve(): Unit = ()
  private def helper = 1
}

object App {
  def main(args: Array[String]): Unit = {
    import scala.util.Try
    val builder = new StringBuilder
    import builder._
    println(Try(greet("x")))
  }
}

case class Point(x: Int, y: Int)

enum Color:
  case Red, Green
  def rgb: Int = 0

type Name = String

extension (s: String) def shout: String = s.toUpperCase
