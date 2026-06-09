import 'package:bot_toast/bot_toast.dart';
import 'package:flutter/material.dart';
import 'package:pixez/models/account.dart';
import 'package:pixez/main.dart';
import 'package:pixez/custom/pages/mirror_list_page.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

class SyncSettingsPage extends StatefulWidget {
  const SyncSettingsPage({Key? key}) : super(key: key);

  @override
  State<SyncSettingsPage> createState() => _SyncSettingsPageState();
}

class _SyncSettingsPageState extends State<SyncSettingsPage> {
  final _formKey = GlobalKey<FormState>();
  final _urlController = TextEditingController();
  final _accessTokenController = TextEditingController();
  bool _syncEnabled = false;
  bool _isLoading = false;
  bool _autoMirrorNovels = true;
  int _syncInterval = 3;

  @override
  void initState() {
    super.initState();
    _loadConfig();
  }

  Future<void> _loadConfig() async {
    await SyncConfig.ensureInitialized();
    setState(() {
      _urlController.text = SyncConfig.serverUrl;
      _accessTokenController.text = SyncConfig.accessToken;
      _syncEnabled = SyncConfig.enabled;
      _autoMirrorNovels = SyncConfig.autoMirrorNovels;
      _syncInterval = SyncConfig.syncInterval;
    });
  }

  Future<void> _saveSettings() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isLoading = true;
    });

    final url = _urlController.text.trim();
    final accessToken = _accessTokenController.text.trim();

    // Verify access token first by pinging.
    final ok = await SyncService.ping(url: url, accessToken: accessToken);

    if (ok) {
      SyncConfig.serverUrl = url;
      SyncConfig.accessToken = accessToken;
      SyncConfig.enabled = _syncEnabled;
      SyncConfig.autoMirrorNovels = _autoMirrorNovels;
      SyncConfig.syncInterval = _syncInterval;
      if (_syncEnabled) {
        SyncService.startPeriodicSyncTimer();
      } else {
        SyncService.stopPeriodicSyncTimer();
      }
      BotToast.showText(text: '配置保存并测试成功');
    } else {
      BotToast.showText(text: '无法连接到服务器，配置未保存');
    }

    setState(() {
      _isLoading = false;
    });
  }

  Future<void> _triggerManualSync() async {
    if (SyncConfig.serverUrl.isEmpty) {
      BotToast.showText(text: '请先配置并保存服务器信息');
      return;
    }

    setState(() {
      _isLoading = true;
    });

    try {
      final accountProvider = AccountProvider();
      await accountProvider.open();
      final localAccounts = await accountProvider.getAllAccount();

      if (localAccounts.isEmpty) {
        BotToast.showText(text: '本地未检测到任何账号');
        setState(() {
          _isLoading = false;
        });
        return;
      }

      int successCount = 0;
      BotToast.showText(text: '正在同步 ${localAccounts.length} 个本地账号...');

      for (final account in localAccounts) {
        // Temporarily force enable sync logic in service
        final syncWasEnabled = SyncConfig.enabled;
        SyncConfig.enabled = true;

        final ok = await SyncService.upsertUser(account);
        if (ok) {
          successCount++;
        }

        SyncConfig.enabled = syncWasEnabled;
      }

      BotToast.showText(
        text: '同步完成: 成功 $successCount/${localAccounts.length} 个账号',
      );
    } catch (e) {
      BotToast.showText(text: '同步失败: $e');
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  static const Map<String, String> _tableChineseNames = {
    'ban_comments': '黑名单评论',
    'ban_illusts': '黑名单作品',
    'ban_tags': '黑名单标签',
    'ban_users': '黑名单画师',
    'illust_histories': '插画历史',
    'novel_histories': '小说历史',
    'tag_histories': '标签历史',
  };

  Future<void> _triggerManualUpload() async {
    if (accountStore.now == null) {
      BotToast.showText(text: '当前未登录账户');
      return;
    }
    setState(() {
      _isLoading = true;
    });
    try {
      final syncedTables = await SyncService.backupUserDataSelective(
        accountStore.now!.userId,
      );
      if (syncedTables != null) {
        if (syncedTables.isEmpty) {
          BotToast.showText(text: '备份成功：云端数据已是最新，无需同步');
        } else {
          final tableNames = syncedTables
              .map((t) => _tableChineseNames[t] ?? t)
              .join('、');
          BotToast.showText(text: '备份成功！已增量同步变动表：$tableNames');
        }
      } else {
        BotToast.showText(text: '备份上报失败，请检查连接或配置');
      }
    } catch (e) {
      BotToast.showText(text: '备份上报失败: $e');
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  Future<void> _triggerManualRestore() async {
    if (accountStore.now == null) {
      BotToast.showText(text: '当前未登录账户');
      return;
    }
    setState(() {
      _isLoading = true;
    });
    try {
      final syncedTables = await SyncService.restoreUserDataSelective(
        accountStore.now!.userId,
      );
      if (syncedTables != null) {
        if (syncedTables.isEmpty) {
          BotToast.showText(text: '恢复成功：本地数据与云端完全一致，无需同步');
        } else {
          final tableNames = syncedTables
              .map((t) => _tableChineseNames[t] ?? t)
              .join('、');
          BotToast.showText(text: '恢复成功！已增量下载还原变动表：$tableNames');
        }
      } else {
        BotToast.showText(text: '恢复下载失败，云端可能没有备份或连接出错');
      }
    } catch (e) {
      BotToast.showText(text: '恢复下载失败: $e');
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  @override
  void dispose() {
    _urlController.dispose();
    _accessTokenController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('数据同步设置')),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : SingleChildScrollView(
              padding: const EdgeInsets.all(20.0),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // Enable Sync Toggle
                    Card(
                      elevation: 2,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Column(
                        children: [
                          SwitchListTile(
                            title: const Text('启用云端同步'),
                            subtitle: const Text('自动将登录凭证及屏蔽/历史等数据同步至后端'),
                            value: _syncEnabled,
                            onChanged: (val) {
                              setState(() {
                                _syncEnabled = val;
                              });
                            },
                          ),
                          if (_syncEnabled)
                            ListTile(
                              title: const Text('自动同步频率'),
                              trailing: DropdownButton<int>(
                                value: _syncInterval,
                                items: const [
                                  DropdownMenuItem(
                                    value: 1,
                                    child: Text('每 1 分钟'),
                                  ),
                                  DropdownMenuItem(
                                    value: 3,
                                    child: Text('每 3 分钟 (默认)'),
                                  ),
                                  DropdownMenuItem(
                                    value: 5,
                                    child: Text('每 5 分钟'),
                                  ),
                                  DropdownMenuItem(
                                    value: 10,
                                    child: Text('每 10 分钟'),
                                  ),
                                  DropdownMenuItem(
                                    value: 30,
                                    child: Text('每 30 分钟'),
                                  ),
                                  DropdownMenuItem(
                                    value: 60,
                                    child: Text('每 60 分钟'),
                                  ),
                                ],
                                onChanged: (val) {
                                  if (val != null) {
                                    setState(() {
                                      _syncInterval = val;
                                    });
                                  }
                                },
                              ),
                            ),
                          SwitchListTile(
                            title: const Text('自动镜像小说'),
                            subtitle: const Text('打开小说详情时自动加入镜像队列'),
                            value: _autoMirrorNovels,
                            onChanged: (val) {
                              setState(() {
                                _autoMirrorNovels = val;
                              });
                            },
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 16),
                    // Server credentials card
                    Card(
                      elevation: 2,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.all(16.0),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '服务器配置',
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                                color: theme.primaryColor,
                              ),
                            ),
                            const SizedBox(height: 16),
                            TextFormField(
                              controller: _urlController,
                              decoration: const InputDecoration(
                                labelText: '服务器地址',
                                hintText: 'http://192.168.1.100:8080',
                                prefixIcon: Icon(Icons.dns),
                                border: OutlineInputBorder(),
                                isDense: true,
                              ),
                              validator: (val) {
                                if (val == null || val.trim().isEmpty) {
                                  return '服务器地址不能为空';
                                }
                                if (!val.startsWith('http://') &&
                                    !val.startsWith('https://')) {
                                  return '必须以 http:// 或 https:// 开头';
                                }
                                return null;
                              },
                            ),
                            const SizedBox(height: 16),
                            TextFormField(
                              controller: _accessTokenController,
                              obscureText: true,
                              decoration: const InputDecoration(
                                labelText: 'AccessToken',
                                hintText: '在 PixezServer 设置中创建的访问令牌',
                                prefixIcon: Icon(Icons.vpn_key),
                                border: OutlineInputBorder(),
                                isDense: true,
                              ),
                              validator: (val) =>
                                  (val == null || val.trim().isEmpty)
                                  ? 'AccessToken 不能为空'
                                  : null,
                            ),
                            const SizedBox(height: 20),
                            Row(
                              mainAxisAlignment: MainAxisAlignment.end,
                              children: [
                                ElevatedButton(
                                  onPressed: _saveSettings,
                                  child: const Text('测试并保存'),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: 24),
                    // Manual sync utilities
                    Card(
                      elevation: 2,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.all(16.0),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '同步工具',
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                            const SizedBox(height: 8),
                            Text(
                              '手动将本地保存的所有账户上报至服务器。在首次配置完服务器，或者新设备需要上传本地历史账户时使用。',
                              style: theme.textTheme.bodyMedium?.copyWith(
                                color: theme.hintColor,
                              ),
                            ),
                            const SizedBox(height: 16),
                            ElevatedButton.icon(
                              onPressed: _triggerManualSync,
                              icon: const Icon(Icons.upload),
                              label: const Text('立即同步本地所有账户'),
                              style: ElevatedButton.styleFrom(
                                backgroundColor:
                                    theme.colorScheme.secondaryContainer,
                                foregroundColor:
                                    theme.colorScheme.onSecondaryContainer,
                              ),
                            ),
                            const Divider(height: 32),
                            Text(
                              '当前账户数据备份与恢复',
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                            const SizedBox(height: 8),
                            Text(
                              '备份当前登录账户的本地屏蔽列表、历史浏览记录与搜索标签至服务器，或者从服务器下载备份覆盖恢复至本地（先删后插）。',
                              style: theme.textTheme.bodyMedium?.copyWith(
                                color: theme.hintColor,
                              ),
                            ),
                            const SizedBox(height: 16),
                            Wrap(
                              spacing: 12,
                              runSpacing: 12,
                              children: [
                                ElevatedButton.icon(
                                  onPressed: _triggerManualUpload,
                                  icon: const Icon(Icons.cloud_upload),
                                  label: const Text('备份当前账户数据'),
                                  style: ElevatedButton.styleFrom(
                                    backgroundColor: theme.primaryColor,
                                    foregroundColor: Colors.white,
                                  ),
                                ),
                                ElevatedButton.icon(
                                  onPressed: _triggerManualRestore,
                                  icon: const Icon(Icons.cloud_download),
                                  label: const Text('恢复当前账户数据'),
                                  style: ElevatedButton.styleFrom(
                                    backgroundColor:
                                        theme.colorScheme.tertiaryContainer,
                                    foregroundColor:
                                        theme.colorScheme.onTertiaryContainer,
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: 16),
                    // Mirror management
                    Card(
                      elevation: 2,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: ListTile(
                        leading: const Icon(Icons.collections),
                        title: const Text('查看镜像'),
                        subtitle: const Text('查看和管理已镜像的插画与小说'),
                        trailing: const Icon(Icons.chevron_right),
                        onTap: () {
                          Navigator.of(context).push(
                            MaterialPageRoute(
                              builder: (_) => const MirrorListPage(),
                            ),
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
            ),
    );
  }
}
