#!/usr/bin/env dart
// The app's entry point.
@TestOn('vm')
library shop_app;

import 'dart:async';
import "dart:io" as io;
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http show get;
import 'package:shop_app/src/model.dart';
import 'package:shop_core/shop_core.dart';
import 'package:path/path.dart' as p;
import 'package:missing/missing.dart' deferred as missing;
import 'package:provider/provider.dart';
import 'package:tracing/tracing.dart';
import 'package:nightly/nightly.dart';
import 'package:private_kit/private_kit.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'src/platform.dart'
    if (dart.library.io) 'src/platform_io.dart'
    if (dart.library.js_interop) 'src/platform_web.dart';
import 'https://example.com/remote.dart';
export 'src/model.dart' show Product;

part 'main.g.dart';

/* A /* nested */ comment with import 'not/an/import.dart'; */
void main() {
  runApp(const ShopApp());
}

class ShopApp extends StatelessWidget {
  const ShopApp({super.key});

  @override
  Widget build(BuildContext context) => MaterialApp(
        home: Scaffold(body: Text('${io.Platform.version} {not a brace')),
      );
}
