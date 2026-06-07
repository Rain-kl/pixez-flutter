import 'package:bot_toast/bot_toast.dart';
import 'package:flutter/material.dart';
import 'package:pixez/custom/services/sync_api.dart';
import 'package:pixez/er/leader.dart';
import 'package:pixez/page/novel/viewer/novel_viewer.dart';
import 'package:pixez/page/picture/illust_lighting_page.dart';
import 'package:pixez/page/picture/illust_store.dart';

class MirrorListPage extends StatefulWidget {
  const MirrorListPage({Key? key}) : super(key: key);

  @override
  State<MirrorListPage> createState() => _MirrorListPageState();
}

class _MirrorListPageState extends State<MirrorListPage>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  int _illustPage = 1;
  int _novelPage = 1;
  int _illustTotal = 0;
  int _novelTotal = 0;
  List<Map<String, dynamic>> _illustItems = [];
  List<Map<String, dynamic>> _novelItems = [];
  bool _isLoading = false;
  final Set<int> _selectedIllusts = {};
  final Set<int> _selectedNovels = {};
  bool _selectMode = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _tabController.addListener(() {
      if (!_tabController.indexIsChanging) setState(() {});
    });
    _loadData();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  bool get _isIllustTab => _tabController.index == 0;

  Set<int> get _currentSelection =>
      _isIllustTab ? _selectedIllusts : _selectedNovels;

  Future<void> _loadData() async {
    if (_isLoading) return;
    setState(() => _isLoading = true);
    try {
      final results = await Future.wait([
        SyncApi.listMirroredIllusts(page: _illustPage),
        SyncApi.listMirroredNovels(page: _novelPage),
      ]);
      if (mounted) {
        setState(() {
          if (results[0] != null) {
            _illustItems = List<Map<String, dynamic>>.from(
              results[0]!['items'] ?? [],
            );
            _illustTotal = results[0]!['total'] ?? 0;
          }
          if (results[1] != null) {
            _novelItems = List<Map<String, dynamic>>.from(
              results[1]!['items'] ?? [],
            );
            _novelTotal = results[1]!['total'] ?? 0;
          }
        });
      }
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  Future<void> _refreshCurrentTab() async {
    setState(() => _isLoading = true);
    try {
      if (_isIllustTab) {
        final data = await SyncApi.listMirroredIllusts(page: _illustPage);
        if (data != null && mounted) {
          setState(() {
            _illustItems = List<Map<String, dynamic>>.from(data['items'] ?? []);
            _illustTotal = data['total'] ?? 0;
          });
        }
      } else {
        final data = await SyncApi.listMirroredNovels(page: _novelPage);
        if (data != null && mounted) {
          setState(() {
            _novelItems = List<Map<String, dynamic>>.from(data['items'] ?? []);
            _novelTotal = data['total'] ?? 0;
          });
        }
      }
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  void _goPage(int page) {
    if (_isIllustTab) {
      _illustPage = page;
    } else {
      _novelPage = page;
    }
    _refreshCurrentTab();
  }

  Future<void> _deleteSingle(int id, String type) async {
    final cancel = BotToast.showLoading();
    try {
      bool ok;
      if (type == 'illust') {
        ok = await SyncApi.deleteMirroredIllust(id);
      } else {
        ok = await SyncApi.deleteMirroredNovel(id);
      }
      if (ok) {
        BotToast.showText(text: '已删除');
        _refreshCurrentTab();
      } else {
        BotToast.showText(text: '删除失败');
      }
    } finally {
      cancel();
    }
  }

  Future<void> _batchDelete() async {
    final selection = _currentSelection;
    if (selection.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('确认删除'),
        content: Text('确定要删除选中的 ${selection.length} 条镜像数据吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('删除', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    final cancel = BotToast.showLoading();
    try {
      final targetType = _isIllustTab ? 'illust' : 'novel';
      final ok = await SyncApi.batchDeleteMirrored(
        targetType,
        selection.toList(),
      );
      if (ok) {
        BotToast.showText(text: '已删除 ${selection.length} 条');
        selection.clear();
        _selectMode = false;
        _refreshCurrentTab();
      } else {
        BotToast.showText(text: '批量删除失败');
      }
    } finally {
      cancel();
    }
  }

  void _toggleSelect(int id) {
    final selection = _currentSelection;
    setState(() {
      if (selection.contains(id)) {
        selection.remove(id);
      } else {
        selection.add(id);
      }
      if (selection.isEmpty) _selectMode = false;
    });
  }

  void _selectAll() {
    final items = _isIllustTab ? _illustItems : _novelItems;
    final selection = _currentSelection;
    setState(() {
      if (selection.length == items.length) {
        selection.clear();
        _selectMode = false;
      } else {
        selection.clear();
        for (final item in items) {
          final id =
              (_isIllustTab ? item['illust_id'] : item['novel_id']) as int;
          selection.add(id);
        }
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final items = _isIllustTab ? _illustItems : _novelItems;
    final total = _isIllustTab ? _illustTotal : _novelTotal;
    final currentPage = _isIllustTab ? _illustPage : _novelPage;
    final totalPages = (total / 20).ceil();

    return Scaffold(
      appBar: AppBar(
        title: const Text('镜像管理'),
        bottom: TabBar(
          controller: _tabController,
          tabs: [
            Tab(text: '插画 ($_illustTotal)'),
            Tab(text: '小说 ($_novelTotal)'),
          ],
        ),
        actions: [
          if (_selectMode)
            IconButton(
              icon: Icon(Icons.select_all),
              tooltip: '全选',
              onPressed: _selectAll,
            ),
          IconButton(
            icon: Icon(_selectMode ? Icons.close : Icons.checklist),
            tooltip: _selectMode ? '取消选择' : '批量管理',
            onPressed: () {
              setState(() {
                _selectMode = !_selectMode;
                if (!_selectMode) _currentSelection.clear();
              });
            },
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: _isLoading && items.isEmpty
                ? const Center(child: CircularProgressIndicator())
                : items.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.cloud_off, size: 48, color: theme.hintColor),
                        const SizedBox(height: 8),
                        Text(
                          '暂无镜像数据',
                          style: TextStyle(color: theme.hintColor),
                        ),
                      ],
                    ),
                  )
                : RefreshIndicator(
                    onRefresh: _refreshCurrentTab,
                    child: ListView.builder(
                      itemCount: items.length,
                      itemBuilder: (context, index) {
                        final item = items[index];
                        final id =
                            (_isIllustTab
                                    ? item['illust_id']
                                    : item['novel_id'])
                                as int;
                        final status = item['status'] as String? ?? '';
                        final hasMirror = item['has_mirror'] as bool? ?? false;
                        final updatedAt = item['updated_at'] as String? ?? '';
                        final title = item['title'] as String? ?? '';
                        final userName = item['user_name'] as String? ?? '';
                        final selected = _currentSelection.contains(id);

                        return _buildListItem(
                          context: context,
                          id: id,
                          status: status,
                          hasMirror: hasMirror,
                          updatedAt: updatedAt,
                          selected: selected,
                          type: _isIllustTab ? 'illust' : 'novel',
                          title: title,
                          userName: userName,
                        );
                      },
                    ),
                  ),
          ),
          if (totalPages > 1)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  IconButton(
                    icon: const Icon(Icons.chevron_left),
                    onPressed: currentPage > 1
                        ? () => _goPage(currentPage - 1)
                        : null,
                  ),
                  Text('$currentPage / $totalPages'),
                  IconButton(
                    icon: const Icon(Icons.chevron_right),
                    onPressed: currentPage < totalPages
                        ? () => _goPage(currentPage + 1)
                        : null,
                  ),
                ],
              ),
            ),
        ],
      ),
      bottomNavigationBar: _selectMode && _currentSelection.isNotEmpty
          ? SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: ElevatedButton.icon(
                  onPressed: _batchDelete,
                  icon: const Icon(Icons.delete_forever),
                  label: Text('删除选中 (${_currentSelection.length})'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.red,
                    foregroundColor: Colors.white,
                    minimumSize: const Size.fromHeight(48),
                  ),
                ),
              ),
            )
          : null,
    );
  }

  void _navigateToDetail(int id, String type) {
    if (type == 'illust') {
      Leader.push(
        context,
        IllustLightingPage(id: id, store: IllustStore(id, null)),
      );
    } else {
      Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => NovelViewerPage(id: id, forceMirrorSource: true),
        ),
      );
    }
  }

  Widget _buildListItem({
    required BuildContext context,
    required int id,
    required String status,
    required bool hasMirror,
    required String updatedAt,
    required bool selected,
    required String type,
    required String title,
    required String userName,
  }) {
    final theme = Theme.of(context);

    IconData statusIcon;
    Color statusColor;
    String statusText;
    switch (status) {
      case 'success':
        statusIcon = Icons.cloud_done;
        statusColor = Colors.green;
        statusText = '已镜像';
        break;
      case 'queued':
        statusIcon = Icons.cloud_queue;
        statusColor = Colors.orange;
        statusText = '排队中';
        break;
      case 'processing':
        statusIcon = Icons.cloud_sync;
        statusColor = Colors.blue;
        statusText = '处理中';
        break;
      case 'failed':
        statusIcon = Icons.cloud_off;
        statusColor = Colors.red;
        statusText = '失败';
        break;
      default:
        statusIcon = Icons.cloud;
        statusColor = theme.hintColor;
        statusText = status.isEmpty ? '未知' : status;
    }

    Widget tile = ListTile(
      leading: _selectMode
          ? Checkbox(value: selected, onChanged: (_) => _toggleSelect(id))
          : Icon(statusIcon, color: statusColor),
      title: Text(
        title.isNotEmpty
            ? (userName.isNotEmpty ? '$title - $userName' : title)
            : (type == 'illust' ? '插画 #$id' : '小说 #$id'),
        style: const TextStyle(fontWeight: FontWeight.w500),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        '$statusText  ·  ${updatedAt.replaceAll('T', ' ').substring(0, updatedAt.length > 19 ? 19 : updatedAt.length)}',
        style: TextStyle(color: theme.hintColor, fontSize: 12),
      ),
      trailing: !_selectMode
          ? Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (hasMirror)
                  IconButton(
                    icon: const Icon(Icons.open_in_new, size: 20),
                    tooltip: '查看',
                    onPressed: () => _navigateToDetail(id, type),
                  ),
                IconButton(
                  icon: Icon(
                    Icons.delete_outline,
                    color: Colors.red[300],
                    size: 20,
                  ),
                  tooltip: '删除',
                  onPressed: () => _deleteSingle(id, type),
                ),
              ],
            )
          : null,
      onTap: () {
        if (_selectMode) {
          _toggleSelect(id);
        } else if (hasMirror) {
          _navigateToDetail(id, type);
        }
      },
      onLongPress: () {
        if (!_selectMode) {
          setState(() {
            _selectMode = true;
            _currentSelection.add(id);
          });
        }
      },
    );

    return tile;
  }
}
