import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

class NovelAutoMirrorService {
  static final Set<int> _requestedNovelIds = <int>{};

  static Future<void> enqueueIfEnabled(int novelId) async {
    await SyncConfig.ensureInitialized();
    if (!SyncConfig.enabled ||
        !SyncConfig.autoMirrorNovels ||
        SyncConfig.serverUrl.isEmpty) {
      return;
    }
    if (!_requestedNovelIds.add(novelId)) {
      return;
    }

    try {
      final result = await SyncService.mirrorNovel(novelId);
      if (result == null) {
        _requestedNovelIds.remove(novelId);
      }
    } catch (_) {
      _requestedNovelIds.remove(novelId);
    }
  }
}
