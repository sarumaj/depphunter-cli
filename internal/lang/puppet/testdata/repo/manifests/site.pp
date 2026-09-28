node 'web01.example.com', /^web\d+$/ {
  include role::webserver
}

node default {
  include role::base
  include stdlib
  include apt
  include docker
  include unknownmod::thing
  lookup('classes', Array[String], 'unique').include
}
