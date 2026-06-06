import 'package:pixez/custom/services/sync_api.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/utils/sync_data_utils.dart';

class SyncUserDataService {
  static bool _isSyncing = false;

  static Future<Map<String, String>?> fetchRemoteHashes(String userId) {
    return SyncApi.fetchRemoteHashes(userId);
  }

  static Future<Map<String, String>> computeLocalHashes() {
    return SyncDataUtils.computeLocalHashes();
  }

  static Future<void> syncData(String userId) async {
    if (_isSyncing) return;
    _isSyncing = true;

    try {
      if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
        return;
      }

      final remoteHashes = await fetchRemoteHashes(userId);
      if (remoteHashes == null) {
        return;
      }

      final localHashes = await computeLocalHashes();
      final uploadTables = <String>[];
      final downloadTables = <String>[];

      for (final table in SyncDataUtils.tableNames) {
        final localHash = localHashes[table] ?? 'empty';
        final remoteHash = remoteHashes[table] ?? 'empty';
        final lastSyncedHash = SyncConfig.getLastSyncedHash(userId, table);

        if (localHash == remoteHash) {
          if (lastSyncedHash != localHash) {
            await SyncConfig.setLastSyncedHash(userId, table, localHash);
          }
          continue;
        }

        if (localHash == lastSyncedHash) {
          downloadTables.add(table);
        } else if (remoteHash == lastSyncedHash) {
          uploadTables.add(table);
        } else if (localHash == 'empty' && remoteHash != 'empty') {
          downloadTables.add(table);
        } else {
          uploadTables.add(table);
        }
      }

      if (uploadTables.isNotEmpty) {
        final success = await uploadSelectiveUserData(userId, uploadTables);
        if (success) {
          for (final table in uploadTables) {
            await SyncConfig.setLastSyncedHash(
              userId,
              table,
              localHashes[table] ?? 'empty',
            );
          }
        }
      }

      if (downloadTables.isNotEmpty) {
        final success = await downloadAndRestoreSelectiveUserData(
          userId,
          downloadTables,
        );
        if (success) {
          final newLocalHashes = await computeLocalHashes();
          for (final table in downloadTables) {
            await SyncConfig.setLastSyncedHash(
              userId,
              table,
              newLocalHashes[table] ?? 'empty',
            );
          }
        }
      }

      SyncConfig.lastSyncTimestamp = DateTime.now().millisecondsSinceEpoch;
    } catch (_) {
      // Background sync should not interrupt the host app.
    } finally {
      _isSyncing = false;
    }
  }

  static Future<List<String>?> backupUserDataSelective(String userId) async {
    try {
      if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
        return null;
      }

      final remoteHashes = await fetchRemoteHashes(userId);
      if (remoteHashes == null) {
        return null;
      }

      final localHashes = await computeLocalHashes();
      final uploadTables = _changedTables(localHashes, remoteHashes);

      if (uploadTables.isEmpty) {
        return [];
      }

      final success = await uploadSelectiveUserData(userId, uploadTables);
      if (!success) {
        return null;
      }

      for (final table in uploadTables) {
        await SyncConfig.setLastSyncedHash(
          userId,
          table,
          localHashes[table] ?? 'empty',
        );
      }
      SyncConfig.lastSyncTimestamp = DateTime.now().millisecondsSinceEpoch;
      return uploadTables;
    } catch (_) {
      return null;
    }
  }

  static Future<List<String>?> restoreUserDataSelective(String userId) async {
    try {
      if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
        return null;
      }

      final remoteHashes = await fetchRemoteHashes(userId);
      if (remoteHashes == null) {
        return null;
      }

      final localHashes = await computeLocalHashes();
      final downloadTables = _changedTables(localHashes, remoteHashes);

      if (downloadTables.isEmpty) {
        return [];
      }

      final success = await downloadAndRestoreSelectiveUserData(
        userId,
        downloadTables,
      );
      if (!success) {
        return null;
      }

      final newLocalHashes = await computeLocalHashes();
      for (final table in downloadTables) {
        await SyncConfig.setLastSyncedHash(
          userId,
          table,
          newLocalHashes[table] ?? 'empty',
        );
      }
      SyncConfig.lastSyncTimestamp = DateTime.now().millisecondsSinceEpoch;
      return downloadTables;
    } catch (_) {
      return null;
    }
  }

  static Future<bool> uploadUserData(String userId) async {
    final success = await uploadSelectiveUserData(userId, null);
    if (success) {
      final localHashes = await computeLocalHashes();
      for (final entry in localHashes.entries) {
        await SyncConfig.setLastSyncedHash(userId, entry.key, entry.value);
      }
      SyncConfig.lastSyncTimestamp = DateTime.now().millisecondsSinceEpoch;
    }
    return success;
  }

  static Future<bool> uploadSelectiveUserData(
    String userId,
    List<String>? selectTables,
  ) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return false;
    }

    try {
      final payload = await SyncDataUtils.collectLocalUserData(selectTables);
      return SyncApi.uploadSyncData(userId, payload);
    } catch (_) {
      return false;
    }
  }

  static Future<bool> downloadAndRestoreUserData(String userId) async {
    final success = await downloadAndRestoreSelectiveUserData(userId, null);
    if (success) {
      final localHashes = await computeLocalHashes();
      for (final entry in localHashes.entries) {
        await SyncConfig.setLastSyncedHash(userId, entry.key, entry.value);
      }
      SyncConfig.lastSyncTimestamp = DateTime.now().millisecondsSinceEpoch;
    }
    return success;
  }

  static Future<bool> downloadAndRestoreSelectiveUserData(
    String userId,
    List<String>? selectTables,
  ) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return false;
    }

    try {
      final data = await SyncApi.downloadSyncData(userId, selectTables);
      if (data == null) {
        return false;
      }

      await SyncDataUtils.restoreUserData(data);
      return true;
    } catch (_) {
      return false;
    }
  }

  static List<String> _changedTables(
    Map<String, String> localHashes,
    Map<String, String> remoteHashes,
  ) {
    final tables = <String>[];
    for (final table in SyncDataUtils.tableNames) {
      if ((localHashes[table] ?? 'empty') != (remoteHashes[table] ?? 'empty')) {
        tables.add(table);
      }
    }
    return tables;
  }
}
