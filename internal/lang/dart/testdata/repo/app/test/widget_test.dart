import 'package:flutter_test/flutter_test.dart';
import 'package:shop_app/main.dart';
import 'package:lints/lints.dart';

void main() {
  testWidgets('starts', (tester) async {
    await tester.pumpWidget(const ShopApp());
  });
}
