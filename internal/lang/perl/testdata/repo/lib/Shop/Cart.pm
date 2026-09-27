package Shop::Cart;
use parent -norequire, 'Shop::Base';
use base qw(Exporter);

sub new {
    my $class = shift;
    return bless {}, $class;
}

sub add ($self, $item) {
    push @{ $self->{items} }, $item;
}

sub mymax(\@;$);

package Shop::Cart::Item {
    use Try::Tiny;
    sub price { 1 }
}

sub count { scalar @_ }
1;
