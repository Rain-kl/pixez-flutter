import 'package:dio/io.dart';

class ConversionLayerAdapter extends IOHttpClientAdapter {
  final dynamic client;
  ConversionLayerAdapter(this.client);
}
