import 'dart:convert';
import 'package:pixez/custom/services/sync_config.dart';

class SyncUtils {
  /// Appends the sync server's Basic Authorization header to the provided [headers]
  /// if the request [url] is targeting the enabled sync server.
  static void addAuthHeaderIfNeeded(String? url, Map<String, String> headers) {
    if (url != null &&
        SyncConfig.enabled &&
        SyncConfig.serverUrl.isNotEmpty &&
        url.startsWith(SyncConfig.serverUrl)) {
      final String auth = 'Basic ' +
          base64Encode(utf8.encode('${SyncConfig.username}:${SyncConfig.password}'));
      headers['Authorization'] = auth;
    }
  }
}
