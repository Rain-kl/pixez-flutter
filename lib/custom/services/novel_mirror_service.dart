import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:mobx/mobx.dart';
import 'package:pixez/custom/services/sync_service.dart';
import 'package:pixez/main.dart';
import 'package:pixez/models/novel_recom_response.dart';
import 'package:pixez/models/novel_web_response.dart';
import 'package:pixez/page/novel/viewer/novel_store.dart';

/// Service for loading novel content from the mirror server.
///
/// Instead of modifying NovelStore directly, this service fetches mirrored
/// data and updates the store's observable properties from the outside.
/// This keeps the original novel_store.dart untouched.
class NovelMirrorService {
  /// Fetch novel text + detail from mirror and update the store.
  ///
  /// Returns true if loading succeeded, false otherwise.
  static Future<bool> fetchFromMirror(NovelStore store) async {
    try {
      final textResponse = await SyncService.getMirroredNovelText(store.id);
      final textData = _decodeMap(textResponse?.data);
      if (textResponse?.statusCode != 200 || textData == null) {
        runInAction(() {
          store.errorMessage = '从镜像加载小说内容失败';
        });
        return false;
      }
      final novelTextResponse = NovelWebResponse.fromJson(textData);
      final spans = await compute(buildSpans, novelTextResponse);

      final detailResponse = await SyncService.getMirroredNovelDetail(store.id);
      final detailData = _decodeMap(detailResponse?.data);
      final novelData = detailData?['novel'];
      if (detailResponse?.statusCode != 200 ||
          novelData is! Map<String, dynamic>) {
        runInAction(() {
          store.errorMessage = '从镜像加载小说详情失败';
        });
        return false;
      }
      final novel = Novel.fromJson(novelData);

      runInAction(() {
        store.errorMessage = null;
        store.bookedOffset = 0.0;
        store.novelTextResponse = novelTextResponse;
        store.spans = spans;
        store.novel = novel;
      });
      novelHistoryStore.insert(novel);
      store.fetchOffset();
      return true;
    } catch (e) {
      print(e);
      runInAction(() {
        store.errorMessage = e.toString();
      });
      return false;
    }
  }

  static Map<String, dynamic>? _decodeMap(dynamic data) {
    if (data is Map<String, dynamic>) {
      return data;
    }
    if (data is String) {
      final decoded = jsonDecode(data);
      if (decoded is Map<String, dynamic>) {
        return decoded;
      }
    }
    return null;
  }
}
