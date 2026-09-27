use v5.38;
use experimental 'class';

class Shop::Point 1.0 :isa(Shop::Base) {
    field $x :param = 0;
    method coords { return ($x) }
}
