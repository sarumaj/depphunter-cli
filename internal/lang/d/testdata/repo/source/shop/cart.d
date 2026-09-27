module shop.cart;

import shop.models.user;

enum TaxRate = 0.2;
enum int MaxItems = 99, MinItems = 1;
enum isItem(T) = is(T == struct);
enum Currency : string { eur = "EUR", usd = "USD" }

alias Price = long;
alias long Quantity;

interface Priced
{
	Price price() const;
}

class Cart : Priced
{
	struct Line
	{
		string sku;
		Quantity qty;
		Price total() const { return 0; }
	}

	private Line[] lines;
	alias lines this;
	enum limit = 10;

	this(User owner) { }
	~this() { }

	@property size_t length() const nothrow @safe { return lines.length; }

	Price price() const
	in (lines.length < MaxItems)
	out (r; r >= 0)
	do
	{
		auto sum = () { return 0L; };
		return sum();
	}

	void add(T)(T item) if (isItem!T)
	{
		struct Hidden { }
		lines ~= Line(item.sku, 1);
	}
}

union Amount
{
	long cents;
	double value;
}

template Box(T)
{
	struct Box { T value; }
}

mixin template Logged()
{
	void log(string msg) { }
}

private:

version (Windows)
{
	void platform() { }
}
else
{
	void platform() { }
}

static if (is(Price == long))
	Price zero() { return 0; }

@safe nothrow:

auto total(Cart c) => c.price();

extern (C) int shop_version();

unittest
{
	struct Local { }
	import std.exception : assertThrown;
}
