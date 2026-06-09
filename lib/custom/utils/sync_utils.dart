import 'package:pixez/custom/services/sync_config.dart';

class SyncUtils {
  /// Appends the sync server's AccessToken header to the provided [headers]
  /// if the request [url] is targeting the enabled sync server.
  static void addAuthHeaderIfNeeded(String? url, Map<String, String> headers) {
    if (url != null &&
        SyncConfig.enabled &&
        SyncConfig.serverUrl.isNotEmpty &&
        SyncConfig.accessToken.isNotEmpty &&
        url.startsWith(SyncConfig.serverUrl)) {
      headers['Authorization'] = 'Bearer ${SyncConfig.accessToken}';
    }
  }
}
