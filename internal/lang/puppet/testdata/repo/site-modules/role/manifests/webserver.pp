# include fake::comment
class role::webserver {
  include profile::base
  include profile::web, 'profile::db'
  contain profile::app
  require ::profile::base
}
