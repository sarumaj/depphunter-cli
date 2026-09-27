ThisBuild / organization := "com.example"
ThisBuild / scalaVersion := "3.3.3"

val catsVersion = "3.5.4"

lazy val root = (project in file("."))
  .settings(
    name := "app",
    libraryDependencies ++= Seq(
      "org.typelevel" %% "cats-effect" % catsVersion,
      "io.circe" %% "circe-core" % "0.14.+",
      "com.typesafe.akka" %% "akka-actor-typed" % "2.8.5",
      "org.scala-lang.modules" %% "scala-xml" % "2.2.0",
      "org.scalatest" %% "scalatest" % "3.2.18" % Test,
      "com.google.guava" % "guava" % "33.0.0-jre"
    )
  )
