import 'package:flutter/material.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

/// A small badge that appears in the novel detail stats row when the
/// novel has been mirrored to the sync backend.
///
/// If sync is not enabled or the novel is not mirrored, this widget renders
/// as zero-size (invisible).
class NovelMirrorBadge extends StatefulWidget {
  final int novelId;

  const NovelMirrorBadge({super.key, required this.novelId});

  @override
  State<NovelMirrorBadge> createState() => _NovelMirrorBadgeState();
}

class _NovelMirrorBadgeState extends State<NovelMirrorBadge> {
  bool? _mirrored;

  @override
  void initState() {
    super.initState();
    _checkMirrorStatus();
  }

  Future<void> _checkMirrorStatus() async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return;
    }
    try {
      final result = await SyncService.batchCheckNovelMirror([widget.novelId]);
      if (mounted) {
        setState(() {
          _mirrored = result.contains(widget.novelId);
        });
      }
    } catch (_) {
      // Silently ignore — badge is purely informational.
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_mirrored != true) {
      return const SizedBox.shrink();
    }
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(width: 4.0),
        Icon(
          Icons.cloud_done,
          color: Colors.green,
          size: 13.0,
        ),
      ],
    );
  }
}
