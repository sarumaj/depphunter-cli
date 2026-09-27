#!perl -T
use strict;
use Test::More;
use Test::Deep;
use FindBin;
use lib "$FindBin::Bin/testlib";
use lib 'inc';
use Shop::Inc;
use TestHelper;
use Shop;
use JSON::MaybeXS;

do "$FindBin::Bin/data/fixture.pl";
require "t/data/fixture.pl";
require "missing.pl";
use_ok('Shop::Cart');
done_testing;
