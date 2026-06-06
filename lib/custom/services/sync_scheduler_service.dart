import 'dart:async';

import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_user_data_service.dart';
import 'package:pixez/main.dart';

class SyncSchedulerService {
  static Timer? _periodicTimer;

  static void startPeriodicSyncTimer() {
    _periodicTimer?.cancel();
    _periodicTimer = Timer.periodic(const Duration(minutes: 1), (_) async {
      if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
        return;
      }

      final activeAccount = accountStore.now;
      if (activeAccount == null) return;

      final userId = activeAccount.userId.toString();
      final now = DateTime.now().millisecondsSinceEpoch;
      final lastSync = SyncConfig.lastSyncTimestamp;
      final intervalMs = SyncConfig.syncInterval * 60 * 1000;

      if (now - lastSync >= intervalMs) {
        await SyncUserDataService.syncData(userId);
      }
    });
  }

  static void stopPeriodicSyncTimer() {
    _periodicTimer?.cancel();
    _periodicTimer = null;
  }
}
