use Module::Build;
Module::Build->new(
    module_name    => 'Acme::Built',
    requires       => { 'Path::Tiny' => '0.100' },
    build_requires => { 'Test::More' => 0 },
)->create_build_script;
