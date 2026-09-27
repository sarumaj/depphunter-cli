use Plack::Builder;
use Shop;
builder { mount '/' => Shop->new };
