import '../shop_core.dart';

class Cart {
  final List<String> items = [];
  List<String> get slugs => sortedSlugs(items);
}
