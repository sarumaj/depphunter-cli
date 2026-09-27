/// The shop's domain.
library;

import 'package:collection/collection.dart';
import 'package:shop_utils/shop_utils.dart' show slug;

export 'src/cart.dart';

List<String> sortedSlugs(Iterable<String> names) => names.map(slug).sorted();
