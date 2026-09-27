import 'package:collection/collection.dart';
import 'package:intl/intl.dart';
import 'package:meta/meta.dart';

part 'model.part.dart';

typedef Json = Map<String, Object?>;
typedef void Listener(Product p);

const currency = 'EUR';
final formatter = NumberFormat.currency(name: currency);
var counter = 0, other = 1;
late String Function(int) describe;

int total(Iterable<Product> ps) => ps.map((p) => p.cents).sum;
Future<void> save<T extends Object>(T value) async {}
String get greeting => 'hi';
set greeting(String v) {}

@immutable
abstract base class Entity {
  const Entity(this.id);
  final String id;
}

final class Product extends Entity with Priced implements Comparable<Product> {
  Product(super.id, this.cents, {this.tags = const {}});
  Product.free(String id) : this(id, 0);
  factory Product.fromJson(Json json) => Product(json['id'] as String, json['cents'] as int);

  static const zero = 0;
  @override
  final int cents;
  final Set<String> tags;
  Map<String, int> counts = {'a': 1};
  void Function()? onChange;

  @override
  int compareTo(Product other) => cents - other.cents;
  bool operator ==(Object other) => other is Product && other.id == id;
  int operator [](int i) => i;
  void operator []=(int i, int v) {}
  String get label {
    return '$id: ${cents / 100} $currency';
  }

  set label(String v) {}
  (int, String) pair() => (cents, id);
}

mixin Priced {
  int get cents;
  String price() => '$cents';
}

mixin class Tracked {}

sealed class Shape {}

enum Status {
  active('A'),
  archived('X');

  const Status(this.code);
  final String code;
  bool get isActive => this == Status.active;
}

enum Plain { a, b }

extension ProductList on List<Product> {
  int get cents => total(this);
}

extension on int {
  int get doubled => this * 2;
}

extension type const ProductId(String value) implements String {
  ProductId.parse(String s) : value = s.trim();
  bool get isEmpty => value.isEmpty;
}
