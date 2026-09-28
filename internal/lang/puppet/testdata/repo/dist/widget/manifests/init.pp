class widget (
  Stdlib::Absolutepath $dir = '/opt/widget',
) {
  include widget::config
  archive { '/tmp/w.tgz': source => 'https://example.com/w.tgz' }
  file_line { 'x': path => '/etc/x', line => 'y' }
  stdlib::ensure_packages(['curl'])
  $t = template('widget/conf.erb')
  $f = file('widget/data.txt')
}
