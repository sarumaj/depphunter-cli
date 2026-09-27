package Shop;
use strict;
use warnings;
use v5.16;
use Moose;
use Shop::Cart;
use Plack::Request;
use LWP::UserAgent;
use Scalar::Util qw(blessed);
use Data::Dumper;
use POSIX ();
use Mojo::UserAgent;
use DateTime::Format::ISO8601;
use Some::Unknown::Thing 1.2 qw(x);
use constant { TAX => 0.2, CURRENCY => 'EUR' };
use constant LIMIT => 10;

with 'Shop::Role::Priced';
extends 'Shop::Base';

has 'name' => (is => 'ro');
has [qw(price qty)] => (is => 'rw');
has '+id' => (default => 1);

sub total {
    my ($self) = @_;
    my $avg = $self->price / 2; # a division, not a pattern: use Fake::Division;
    return $avg * $self->qty;
}

sub _helper {
    my $text = <<"EOT" . <<'RAW';
use Fake::Heredoc;
EOT
require Fake::Raw;
RAW
    my $q = q{use Fake::Q; { nested }};
    my $re = qr{use Fake::Re}x;
    (my $s = $text) =~ s{use}
                        {no}g;
    $text =~ tr/a-z/A-Z/;
    my @w = qw/use Fake::QW/;
    my %h = (s => 1, y => 2, q => 3);
    return $h{s} + $#w + $h{y};
}

eval { require Try::Tiny; 1 };
eval "use JSON::XS; 1";

=head1 SYNOPSIS

  use Fake::Pod;

=cut

1;
__END__
use Fake::End;
