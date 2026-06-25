library rhttp;

class Rhttp {
  static Future<void> init() async {}
}

class RhttpCompatibleClient {
  final ClientSettings? settings;
  RhttpCompatibleClient({this.settings});

  static Future<RhttpCompatibleClient> create({ClientSettings? settings}) async {
    return RhttpCompatibleClient(settings: settings);
  }

  static Future<RhttpCompatibleClient> createSync({ClientSettings? settings}) async {
    return RhttpCompatibleClient(settings: settings);
  }
}

class ClientSettings {
  final bool? enableEch;
  final bool? requireEch;
  final TlsSettings? tlsSettings;
  final DnsSettings? dnsSettings;

  ClientSettings({
    this.enableEch,
    this.requireEch,
    this.tlsSettings,
    this.dnsSettings,
  });
}

class TlsSettings {
  final bool? verifyCertificates;
  final bool? sni;
  TlsSettings({this.verifyCertificates, this.sni});
}

class DnsSettings {
  final Map<String, List<String>>? staticOverrides;
  final Future<List<String>> Function(String)? dynamicResolver;

  DnsSettings._({this.staticOverrides, this.dynamicResolver});

  static DnsSettings static({required Map<String, List<String>> overrides}) {
    return DnsSettings._(staticOverrides: overrides);
  }

  static DnsSettings dynamic({required Future<List<String>> Function(String host) resolver}) {
    return DnsSettings._(dynamicResolver: resolver);
  }
}
