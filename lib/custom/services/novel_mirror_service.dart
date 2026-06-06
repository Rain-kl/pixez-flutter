import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:pixez/custom/services/sync_service.dart';
import 'package:pixez/main.dart';
import 'package:pixez/models/novel_recom_response.dart';
import 'package:pixez/models/novel_web_response.dart';
import 'package:pixez/page/novel/viewer/image_text.dart';
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
    store.errorMessage = null;
    try {
      store.bookedOffset = 0.0;

      // Fetch novel text from mirror (webview JSON = NovelWebResponse)
      final textResponse = await SyncService.getMirroredNovelText(store.id);
      if (textResponse != null &&
          textResponse.statusCode == 200 &&
          textResponse.data != null) {
        final textData = textResponse.data is String
            ? jsonDecode(textResponse.data)
            : textResponse.data as Map<String, dynamic>;
        store.novelTextResponse = NovelWebResponse.fromJson(textData);
        store.spans =
            await compute(buildSpans, store.novelTextResponse!);
      } else {
        store.errorMessage = '从镜像加载小说内容失败';
        return false;
      }

      // Fetch novel detail from mirror
      if (store.novel == null) {
        final detailResponse =
            await SyncService.getMirroredNovelDetail(store.id);
        if (detailResponse != null &&
            detailResponse.statusCode == 200 &&
            detailResponse.data != null) {
          final detailData = detailResponse.data is String
              ? jsonDecode(detailResponse.data)
              : detailResponse.data;
          store.novel = Novel.fromJson(detailData['novel']);
        }
      }

      if (store.novel != null) {
        novelHistoryStore.insert(store.novel!);
      }
      store.fetchOffset();
      return true;
    } catch (e) {
      print(e);
      store.errorMessage = e.toString();
      return false;
    }
  }
}
