import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/er/lprinter.dart';

class SyncUtils {
  /// Appends the sync server's AccessToken header to the provided [headers]
  /// if the request [url] is targeting the enabled sync server.
  static void addAuthHeaderIfNeeded(String? url, Map<String, String> headers) {
    final serverUrl = SyncConfig.serverUrl;
    final urlMatchesServer =
        url != null && serverUrl.isNotEmpty && url.startsWith(serverUrl);
    final shouldAddAuth =
        urlMatchesServer &&
        SyncConfig.enabled &&
        SyncConfig.accessToken.isNotEmpty;

    LPrinter.d(
      '[SyncAuth] url=$url serverUrl=$serverUrl '
      'urlMatchesServer=$urlMatchesServer enabled=${SyncConfig.enabled} '
      'tokenConfigured=${SyncConfig.accessToken.isNotEmpty} '
      'authorizationAdded=$shouldAddAuth',
    );

    if (shouldAddAuth) {
      headers['Authorization'] = 'Bearer ${SyncConfig.accessToken}';
    }
  }
}
