# A profile using every reference form; include commented::out
/* include commented::block */
class profile::web (
  Stdlib::Port $port = 80,
  Profile::Port $admin = 8080,
  Optional[String] $name = undef,
) inherits profile::params {
  include apache
  include apache::mod::ssl
  class { 'apache::mod::php': }
  apache::vhost { 'site':
    port    => $port,
    require => Class['profile::base'],
    before  => Apache::Vhost['other'],
  }
  file { '/etc/motd':
    content => epp('profile/motd.epp', { 'x' => 1 }),
    source  => 'puppet:///modules/profile/banner.txt',
  }
  package { 'nginx': ensure => installed }
  service { 'nginx': require => Package['nginx'] }
  exec { 'x': command => "/bin/echo include fake::dq ${profile::greet('a')}" }
  $msg = @("END"/L)
    include fake::heredoc
    apache::fake { 'x': }
    | END
  notify { $msg: }
  $single = 'include fake::sq'
  $x = profile::greet('world')
  $y = profile::shout('x')
  $z = merge({}, {})
  $w = [1, 2].map |$v| { $v * 2 }
  ini_setting { 'x': ensure => present }
  concat::fragment { 'f': target => '/x' }
  Firewall { ensure => present }
  mysql_user { 'u@localhost': }
  @@nagios_host { $facts['fqdn']: }
  Apache::Vhost <| tag == 'x' |>
  if $x =~ /^include foo::bar$/ { notice('x') }
  $r = 10 / 2
  profile_thing { 'x': }
}
