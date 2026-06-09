import 'package:shared_preferences/shared_preferences.dart';

class SyncConfig {
  static SharedPreferences? _prefs;

  static Future<void> ensureInitialized() async {
    _prefs ??= await SharedPreferences.getInstance();
  }

  static String get serverUrl {
    final url = _prefs?.getString('sync_server_url') ?? '';
    // Strip trailing slash if present
    if (url.endsWith('/')) {
      return url.substring(0, url.length - 1);
    }
    return url;
  }

  static set serverUrl(String val) {
    _prefs?.setString('sync_server_url', val);
  }

  static String get accessToken => _prefs?.getString('sync_access_token') ?? '';
  static set accessToken(String val) =>
      _prefs?.setString('sync_access_token', val);

  static bool get enabled => _prefs?.getBool('sync_enabled') ?? false;
  static set enabled(bool val) => _prefs?.setBool('sync_enabled', val);

  static bool get autoMirrorNovels =>
      _prefs?.getBool('sync_auto_mirror_novels') ?? true;
  static set autoMirrorNovels(bool val) =>
      _prefs?.setBool('sync_auto_mirror_novels', val);

  static int get syncInterval => _prefs?.getInt('sync_interval_minutes') ?? 3;
  static set syncInterval(int val) =>
      _prefs?.setInt('sync_interval_minutes', val);

  static int get lastSyncTimestamp =>
      _prefs?.getInt('sync_last_timestamp') ?? 0;
  static set lastSyncTimestamp(int val) =>
      _prefs?.setInt('sync_last_timestamp', val);

  static String getLastSyncedHash(String userId, String tableName) {
    return _prefs?.getString('sync_hash_${userId}_$tableName') ?? '';
  }

  static Future<void> setLastSyncedHash(
    String userId,
    String tableName,
    String hash,
  ) async {
    await _prefs?.setString('sync_hash_${userId}_$tableName', hash);
  }
}
