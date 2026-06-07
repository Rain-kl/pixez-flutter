import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

/// In-memory cache of which novel IDs have been mirrored.
///
/// Same pattern as MirrorStatusCache but for novels.
class NovelMirrorStatusCache {
  NovelMirrorStatusCache._();

  static final Set<int> _mirroredIds = {};
  static bool _fetching = false;
  static Timer? _debounceTimer;
  static final Set<int> _pendingIds = {};

  static final ValueNotifier<int> version = ValueNotifier(0);

  static const int _batchSize = 200;
  static const Duration _debounceDelay = Duration(milliseconds: 300);

  static bool isMirrored(int novelId) {
    return _mirroredIds.contains(novelId);
  }

  static void markForCheck(int novelId) {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return;

    _pendingIds.add(novelId);
    _debounceTimer ??= Timer(_debounceDelay, _flush);
  }

  static void _flush() {
    _debounceTimer = null;
    if (_pendingIds.isEmpty) return;
    if (_fetching) {
      _debounceTimer = Timer(_debounceDelay, _flush);
      return;
    }

    final batch =
        _pendingIds.length > _batchSize
            ? _pendingIds.toList().sublist(0, _batchSize)
            : List<int>.from(_pendingIds);
    _pendingIds.removeAll(batch);

    _fetching = true;
    SyncService.batchCheckNovelMirror(batch).then((mirrored) {
      if (mirrored.isNotEmpty) {
        _mirroredIds.addAll(mirrored);
        version.value++;
      }
    }).catchError((_) {}).whenComplete(() {
      _fetching = false;
      if (_pendingIds.isNotEmpty) {
        _debounceTimer = Timer(_debounceDelay, _flush);
      }
    });
  }

  static void invalidateForRefresh() {
    _pendingIds.clear();
    _debounceTimer?.cancel();
    _debounceTimer = null;
  }
}
