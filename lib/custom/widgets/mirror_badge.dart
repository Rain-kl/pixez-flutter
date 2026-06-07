import 'package:flutter/material.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

/// A small badge that appears in the illust detail stats row when the
/// illustration has been mirrored to the sync backend.
///
/// If sync is not enabled or the illust is not mirrored, this widget renders
/// as zero-size (invisible).
class MirrorBadge extends StatefulWidget {
  final int illustId;

  const MirrorBadge({super.key, required this.illustId});

  @override
  State<MirrorBadge> createState() => _MirrorBadgeState();
}

class _MirrorBadgeState extends State<MirrorBadge> {
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
      final result = await SyncService.isIllustMirrored(widget.illustId);
      if (mounted) {
        setState(() {
          _mirrored = result;
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
