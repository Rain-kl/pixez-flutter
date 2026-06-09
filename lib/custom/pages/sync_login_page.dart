import 'package:bot_toast/bot_toast.dart';
import 'package:flutter/material.dart';
import 'package:pixez/er/leader.dart';
import 'package:pixez/main.dart';
import 'package:pixez/models/account.dart';
import 'package:pixez/network/oauth_client.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';

class SyncLoginPage extends StatefulWidget {
  const SyncLoginPage({Key? key}) : super(key: key);

  @override
  State<SyncLoginPage> createState() => _SyncLoginPageState();
}

class _SyncLoginPageState extends State<SyncLoginPage> {
  final _formKey = GlobalKey<FormState>();
  final _urlController = TextEditingController();
  final _accessTokenController = TextEditingController();

  bool _isConnected = false;
  bool _isLoading = false;
  List<dynamic> _users = [];

  @override
  void initState() {
    super.initState();
    _loadSavedConfig();
  }

  Future<void> _loadSavedConfig() async {
    setState(() {
      _isLoading = true;
    });
    await SyncConfig.ensureInitialized();
    _urlController.text = SyncConfig.serverUrl;
    _accessTokenController.text = SyncConfig.accessToken;

    if (_urlController.text.isNotEmpty &&
        _accessTokenController.text.isNotEmpty) {
      // Auto ping to see if we can connect
      _testConnection(silent: true);
    } else {
      setState(() {
        _isLoading = false;
      });
    }
  }

  Future<void> _testConnection({bool silent = false}) async {
    if (!silent) {
      if (!_formKey.currentState!.validate()) return;
    }

    setState(() {
      _isLoading = true;
    });

    final url = _urlController.text.trim();
    final accessToken = _accessTokenController.text.trim();

    final ok = await SyncService.ping(url: url, accessToken: accessToken);

    if (ok) {
      // Save config and enable sync
      SyncConfig.serverUrl = url;
      SyncConfig.accessToken = accessToken;
      SyncConfig.enabled = true;
      SyncService.startPeriodicSyncTimer();

      // Fetch users
      final userList = await SyncService.listUsers();

      setState(() {
        _isConnected = true;
        _users = userList;
        _isLoading = false;
      });

      if (!silent) {
        BotToast.showText(text: '连接成功');
      }
    } else {
      setState(() {
        _isConnected = false;
        _users = [];
        _isLoading = false;
      });
      if (!silent) {
        BotToast.showText(text: '连接失败，请检查配置与网络');
      }
    }
  }

  Future<void> _loginWithUser(Map<String, dynamic> userSummary) async {
    setState(() {
      _isLoading = true;
    });

    final userId = userSummary['pixiv_user_id']?.toString();
    if (userId == null) {
      BotToast.showText(text: '无效的用户ID');
      setState(() {
        _isLoading = false;
      });
      return;
    }

    final fullUser = await SyncService.getUser(userId);
    if (fullUser == null) {
      BotToast.showText(text: '获取用户令牌失败');
      setState(() {
        _isLoading = false;
      });
      return;
    }

    final refreshToken = fullUser['refresh_token']?.toString();
    if (refreshToken == null || refreshToken.isEmpty) {
      BotToast.showText(text: '后端存储的刷新令牌为空');
      setState(() {
        _isLoading = false;
      });
      return;
    }

    try {
      BotToast.showText(text: '正在向 Pixiv 请求新令牌...');
      final response = await oAuthClient.postRefreshAuthToken(
        refreshToken: refreshToken,
      );

      final accountResponse = Account.fromJson(response.data).response;
      final pixivUser = accountResponse.user;

      final accountPersist = AccountPersist(
        userId: pixivUser.id,
        userImage: pixivUser.profileImageUrls.px170x170,
        accessToken: accountResponse.accessToken,
        refreshToken: accountResponse.refreshToken,
        deviceToken: '',
        passWord: 'no more',
        name: pixivUser.name,
        account: pixivUser.account,
        mailAddress: pixivUser.mailAddress,
        isPremium: pixivUser.isPremium ? 1 : 0,
        xRestrict: pixivUser.xRestrict,
        isMailAuthorized: pixivUser.isMailAuthorized ? 1 : 0,
      );

      // Save locally
      final accountProvider = AccountProvider();
      await accountProvider.open();
      await accountProvider.deleteByUserId(pixivUser.id);
      await accountProvider.insert(accountPersist);

      // Trigger sync back to server (report new tokens)
      await SyncService.upsertUser(accountPersist);

      // Fetch global store and navigate home
      await accountStore.fetch();

      // Download and restore user data from cloud (先删后插) in background
      BotToast.showText(text: '正在从云端恢复个人数据...');
      SyncService.downloadAndRestoreUserData(pixivUser.id).then((success) {
        if (success) {
          BotToast.showText(text: '个人数据恢复完成');
          SyncService.startPeriodicSyncTimer();
        }
      });

      BotToast.showText(text: '登录成功');
      Leader.pushUntilHome(context);
    } catch (e) {
      BotToast.showText(text: '登录失败: $e');
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
      appBar: AppBar(
        title: const Text('后端同步登录'),
        elevation: 0,
        backgroundColor: Colors.transparent,
      ),
      extendBodyBehindAppBar: true,
      body: Stack(
        children: [
          // Background Gradient
          Container(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: [
                  theme.colorScheme.surface,
                  theme.primaryColor.withValues(alpha: 0.08),
                ],
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
              ),
            ),
          ),
          SafeArea(
            child: _isLoading
                ? const Center(child: CircularProgressIndicator())
                : Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 24.0),
                    child: Column(
                      children: [
                        const SizedBox(height: 16),
                        _buildConnectionCard(),
                        const SizedBox(height: 24),
                        Expanded(
                          child: _isConnected
                              ? _buildUserList()
                              : _buildIntroPanel(),
                        ),
                      ],
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildConnectionCard() {
    final theme = Theme.of(context);
    return Card(
      elevation: 4,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                '同步服务器配置',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: theme.primaryColor,
                ),
              ),
              const SizedBox(height: 12),
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
              const SizedBox(height: 12),
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
                validator: (val) => (val == null || val.trim().isEmpty)
                    ? 'AccessToken 不能为空'
                    : null,
              ),
              const SizedBox(height: 16),
              ElevatedButton.icon(
                onPressed: () => _testConnection(),
                icon: const Icon(Icons.connect_without_contact),
                label: const Text('连接并加载用户'),
                style: ElevatedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildIntroPanel() {
    final theme = Theme.of(context);
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.cloud_sync,
            size: 80,
            color: theme.primaryColor.withValues(alpha: 0.4),
          ),
          const SizedBox(height: 16),
          Text(
            '尚未连接到同步服务器',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32.0),
            child: Text(
              '输入您的同步后端地址与 AccessToken 并点击连接，即可在此处选择您已备份的 Pixiv 账号快速登录。',
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.hintColor,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildUserList() {
    final theme = Theme.of(context);
    if (_users.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.people_outline, size: 60, color: theme.hintColor),
            const SizedBox(height: 12),
            Text('服务器上没有保存任何用户', style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              '开启设备同步后，登录成功的账号会自动上报到服务器。',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.hintColor,
              ),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 8.0, bottom: 8.0),
          child: Text(
            '选择账号登录 (${_users.length})',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
        ),
        Expanded(
          child: ListView.builder(
            itemCount: _users.length,
            itemBuilder: (context, index) {
              final user = _users[index] as Map<String, dynamic>;
              final name = user['name'] ?? '未知用户';
              final account = user['account'] ?? '';
              final image = user['user_image'] ?? '';
              final id = user['pixiv_user_id'] ?? '';

              return Card(
                margin: const EdgeInsets.symmetric(vertical: 6),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
                child: ListTile(
                  leading: CircleAvatar(
                    backgroundImage: image.isNotEmpty
                        ? NetworkImage(image)
                        : null,
                    child: image.isEmpty ? const Icon(Icons.person) : null,
                  ),
                  title: Text(name),
                  subtitle: Text('@$account ($id)'),
                  trailing: const Icon(Icons.arrow_forward_ios, size: 16),
                  onTap: () => _loginWithUser(user),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}
