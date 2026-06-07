import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

/// In-memory cache of which illust IDs have been mirrored.
///
/// Individual [IllustCard] widgets call [markForCheck] with their own ID.
/// The cache coalesces these into periodic batch requests automatically.
///
/// When batch results arrive, [version] is incremented so that any
/// [ValueListenableBuilder] wrapping [isMirrored] calls will rebuild.
///
/// To follow Pixiv API requests: whenever the official API is queried,
/// we invalidate pending checks so that fresh requests are made on the
/// next page load or refresh.
class MirrorStatusCache {
  MirrorStatusCache._();

  static final Set<int> _mirroredIds = {};
  static bool _fetching = false;
  static Timer? _debounceTimer;
  static final Set<int> _pendingIds = {};

  /// Incremented every time new mirror results are written to [_mirroredIds].
  /// Attach a [ValueListenableBuilder] to this to reactively update badges.
  static final ValueNotifier<int> version = ValueNotifier(0);

  /// Batch size per request.
  static const int _batchSize = 200;

  /// Debounce window to accumulate pending IDs.
  static const Duration _debounceDelay = Duration(milliseconds: 300);

  /// Synchronous lookup — returns true if the illust is known to be mirrored.
  static bool isMirrored(int illustId) {
    return _mirroredIds.contains(illustId);
  }

  /// Register [illustId] for a deferred batch check.
  ///
  /// Call this from IllustCard.initState(). It will not trigger an immediate
  /// network request — IDs are accumulated and sent in batches after a
  /// short debounce window.
  static void markForCheck(int illustId) {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return;

    _pendingIds.add(illustId);
    _debounceTimer ??= Timer(_debounceDelay, _flush);
  }

  static void _flush() {
    _debounceTimer = null;
    if (_pendingIds.isEmpty) return;
    if (_fetching) {
      // Retry after a short delay.
      _debounceTimer = Timer(_debounceDelay, _flush);
      return;
    }

    // Take up to _batchSize IDs from the pending queue.
    final batch =
        _pendingIds.length > _batchSize
            ? _pendingIds.toList().sublist(0, _batchSize)
            : List<int>.from(_pendingIds);
    _pendingIds.removeAll(batch);

    _fetching = true;
    SyncService.batchCheckIllustMirror(batch).then((mirrored) {
      if (mirrored.isNotEmpty) {
        _mirroredIds.addAll(mirrored);
        version.value++; // Notify all listeners.
      }
    }).catchError((_) {
      // Badges are informational only — silently ignore.
    }).whenComplete(() {
      _fetching = false;
      // If more IDs accumulated while we were fetching, flush again.
      if (_pendingIds.isNotEmpty) {
        _debounceTimer = Timer(_debounceDelay, _flush);
      }
    });
  }

  /// Invalidate pending checks to ensure fresh requests on page refresh.
  ///
  /// Call this whenever the official Pixiv API issues a new data request
  /// (e.g., when refreshing bookmarks). This clears the pending queue so
  /// that new cards will queue fresh mirror checks.
  static void invalidateForRefresh() {
    _pendingIds.clear();
    _debounceTimer?.cancel();
    _debounceTimer = null;
  }
}
