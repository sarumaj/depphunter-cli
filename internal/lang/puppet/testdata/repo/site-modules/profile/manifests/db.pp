class profile::db {
  class { 'mysql::server':
    root_password => 'x',
  }
  include ntp
}
