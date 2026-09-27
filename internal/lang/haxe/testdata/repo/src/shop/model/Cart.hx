package shop.model;

import shop.util.Money;

class Cart {
	public function new() {}

	public function total():Float {
		return 0.0;
	}
}

class CartItem {
	public var price:Float;
}

typedef Items = Array<CartItem>;
