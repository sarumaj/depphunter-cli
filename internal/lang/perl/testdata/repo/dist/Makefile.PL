use strict;
use ExtUtils::MakeMaker;

WriteMakefile(
  NAME         => 'Acme::Widget',
  VERSION_FROM => 'lib/Acme/Widget.pm',
  PREREQ_PM    => { 'JSON::PP' => '4.00', 'Moo' => 0 },
  TEST_REQUIRES => { 'Test::Fatal' => '0.016' },
  META_MERGE   => {
    prereqs => { runtime => { recommends => { 'Cpanel::JSON::XS' => '4.0' } } },
  },
);
