import 'package:bot_toast/bot_toast.dart';
import 'package:flutter/material.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';
import 'package:pixez/models/illust.dart';
import 'package:pixez/page/picture/illust_store.dart';

class MirrorListTile extends StatefulWidget {
  final int id;
  final Illusts illusts;
  final IllustStore illustStore;

  const MirrorListTile({
    Key? key,
    required this.id,
    required this.illusts,
    required this.illustStore,
  }) : super(key: key);

  @override
  State<MirrorListTile> createState() => _MirrorListTileState();
}

class _MirrorListTileState extends State<MirrorListTile> {
  bool? _isMirrored;
  bool _isChecking = false;
  String? _taskStatus;

  @override
  void initState() {
    super.initState();
    _checkStatus();
  }

  Future<void> _checkStatus() async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return;
    }
    if (_isChecking) return;
    setState(() {
      _isChecking = true;
    });
    try {
      final res = await SyncService.getIllustMirrorStatus(widget.id);
      if (mounted) {
        setState(() {
          _isMirrored = res?['mirrored'] == true;
          _taskStatus = res?['status']?.toString();
          _isChecking = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _isMirrored = false;
          _isChecking = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return const SizedBox.shrink();
    }

    final isMirrored = _isMirrored;
    final isProcessing = _taskStatus == 'queued' || _taskStatus == 'processing';

    return ListTile(
      leading: Icon(
        isMirrored == true
            ? Icons.cloud_done
            : (isMirrored == null || isProcessing
                  ? Icons.cloud_queue
                  : Icons.cloud_download),
        color: isMirrored == true ? Colors.green : null,
      ),
      title: Text(_titleText(isMirrored, isProcessing)),
      onTap: isMirrored == null
          ? null
          : () async {
              if (isMirrored == true) {
                Navigator.of(context).pop();
                final cancel = BotToast.showLoading();
                try {
                  final response = await SyncService.getMirroredIllustDetail(
                    widget.id,
                  );
                  if (response != null &&
                      response.statusCode == 200 &&
                      response.data != null &&
                      response.data['illust'] != null) {
                    final result = Illusts.fromJson(response.data['illust']);
                    widget.illustStore.illusts = result;
                    widget.illustStore.isBookmark = result.isBookmarked;
                    widget.illustStore.state = result.isBookmarked ? 2 : 0;
                    BotToast.showText(text: "已从镜像加速加载");
                  } else {
                    BotToast.showText(text: "从镜像加载失败");
                  }
                } catch (e) {
                  BotToast.showText(text: "加载失败: $e");
                } finally {
                  cancel();
                }
              } else {
                final cancel = BotToast.showLoading();
                try {
                  final result = await SyncService.mirrorIllust(widget.id);
                  if (result != null) {
                    BotToast.showText(text: "镜像任务已加入队列");
                    if (mounted) {
                      setState(() {
                        _isMirrored = result['mirrored'] == true;
                        _taskStatus = result['status']?.toString() ?? 'queued';
                      });
                    }
                    _pollUntilMirrored();
                  } else {
                    BotToast.showText(text: "镜像入队失败，请检查网络或后端");
                  }
                } catch (e) {
                  BotToast.showText(text: "镜像出错: $e");
                } finally {
                  cancel();
                }
              }
            },
    );
  }

  String _titleText(bool? isMirrored, bool isProcessing) {
    if (isMirrored == null) {
      return "镜像 (检测中...)";
    }
    if (isMirrored) {
      return "已镜像";
    }
    if (isProcessing) {
      return "镜像中";
    }
    if (_taskStatus == 'failed') {
      return "镜像失败";
    }
    return "镜像";
  }

  Future<void> _pollUntilMirrored() async {
    for (var i = 0; i < 12; i++) {
      await Future.delayed(const Duration(seconds: 5));
      if (!mounted) return;
      final res = await SyncService.getIllustMirrorStatus(widget.id);
      if (res == null) continue;
      final mirrored = res['mirrored'] == true;
      final status = res['status']?.toString();
      if (mounted) {
        setState(() {
          _isMirrored = mirrored;
          _taskStatus = status;
        });
      }
      if (mirrored || status == 'failed') {
        return;
      }
    }
  }
}
