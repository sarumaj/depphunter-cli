import utest.Assert;
import shop.model.Cart;

class TestAll {
	static function main() {
		Assert.isTrue(new Cart() != null);
	}
}
