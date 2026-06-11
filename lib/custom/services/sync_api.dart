import 'package:dio/dio.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/er/lprinter.dart';
import 'package:pixez/models/account.dart';

class SyncApi {
  static Dio getDioClient() {
    final dio = Dio(
      BaseOptions(
        baseUrl: SyncConfig.serverUrl,
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 10),
      ),
    );

    dio.options.headers.addAll(_authHeaders(SyncConfig.accessToken));
    return dio;
  }

  static Future<bool> ping({String? url, String? accessToken}) async {
    final targetUrl = url ?? SyncConfig.serverUrl;
    final targetToken = accessToken ?? SyncConfig.accessToken;

    if (targetUrl.isEmpty || targetToken.isEmpty) {
      return false;
    }

    try {
      final dio = Dio(
        BaseOptions(
          baseUrl: targetUrl,
          connectTimeout: const Duration(seconds: 5),
          receiveTimeout: const Duration(seconds: 5),
        ),
      );
      dio.options.headers.addAll(_authHeaders(targetToken));

      final response = await dio.get('/api/pixez/ping');
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Future<List<dynamic>> listUsers() async {
    try {
      final response = await getDioClient().get('/api/pixez/users');
      return (_unwrapData(response) as List<dynamic>?) ?? [];
    } catch (_) {
      return [];
    }
  }

  static Future<Map<String, dynamic>?> getUser(String pixivUserId) async {
    try {
      final response = await getDioClient().get(
        '/api/pixez/users/$pixivUserId',
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<bool> upsertUser(AccountPersist account) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return false;
    }

    try {
      final payload = {
        'name': account.name,
        'account': account.account,
        'mail_address': account.mailAddress,
        'user_image': account.userImage,
        'access_token': account.accessToken,
        'refresh_token': account.refreshToken,
        'device_token': account.deviceToken,
        'is_premium': account.isPremium,
        'x_restrict': account.xRestrict,
        'is_mail_authorized': account.isMailAuthorized,
      };

      final response = await getDioClient().put(
        '/api/pixez/users/${account.userId}',
        data: payload,
      );
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Future<bool> deleteUser(String pixivUserId) async {
    try {
      final response = await getDioClient().delete(
        '/api/pixez/users/$pixivUserId',
      );
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Future<Map<String, String>?> fetchRemoteHashes(String userId) async {
    try {
      final response = await getDioClient().get(
        '/api/pixez/users/$userId/sync-data/hashes',
      );
      final data = _unwrapData(response);
      if (data is Map<String, dynamic>) {
        return data.map((key, value) => MapEntry(key, value.toString()));
      }
      return null;
    } catch (_) {
      return null;
    }
  }

  static Future<bool> uploadSyncData(
    String userId,
    Map<String, dynamic> payload,
  ) async {
    try {
      final response = await getDioClient().post(
        '/api/pixez/users/$userId/sync-data',
        data: payload,
      );
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Future<Map<String, dynamic>?> downloadSyncData(
    String userId,
    List<String>? selectTables,
  ) async {
    try {
      final queryParams = selectTables != null
          ? {'tables': selectTables.join(',')}
          : null;
      final response = await getDioClient().get(
        '/api/pixez/users/$userId/sync-data',
        queryParameters: queryParams,
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<Map<String, dynamic>?> getIllustMirrorStatus(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      final response = await getDioClient().get(
        '/api/pixez/illusts/$id/mirror',
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<Response> getRemovedBookmarkIllusts({
    required int userId,
    int? offset,
    int? limit,
  }) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      throw Exception('Sync server is not configured');
    }

    final response = await getDioClient().get(
      '/api/pixez/users/$userId/bookmarks/illust/removed',
      queryParameters: {
        if (offset != null) 'offset': offset,
        if (limit != null) 'limit': limit,
      },
    );
    return _unwrapPixivPayload(response);
  }

  static Future<Response> getRemovedBookmarkIllustsNext(String url) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      throw Exception('Sync server is not configured');
    }

    final response = await getDioClient().get(url);
    return _unwrapPixivPayload(response);
  }

  static Response _unwrapPixivPayload(Response response) {
    if (_isSuccess(response)) {
      return Response(
        requestOptions: response.requestOptions,
        data: _unwrapData(response),
        statusCode: response.statusCode,
        statusMessage: response.statusMessage,
        headers: response.headers,
        redirects: response.redirects,
        isRedirect: response.isRedirect,
        extra: response.extra,
      );
    }
    throw Exception(_failureMessage(response));
  }

  static Future<bool> isIllustMirrored(int id) async {
    final data = await getIllustMirrorStatus(id);
    return data?['mirrored'] == true;
  }

  static Future<Set<int>> batchCheckIllustMirror(List<int> ids) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty || ids.isEmpty) {
      return {};
    }
    try {
      final response = await getDioClient().post(
        '/api/pixez/illusts/mirror/batch',
        data: {'illust_ids': ids},
      );
      final data = _unwrapData(response);
      if (data is Map<String, dynamic>) {
        final List<dynamic> mirroredIds = data['mirrored_ids'] ?? [];
        return mirroredIds.map((e) => e as int).toSet();
      }
      return {};
    } catch (_) {
      return {};
    }
  }

  static Future<Set<int>> batchCheckNovelMirror(List<int> ids) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty || ids.isEmpty) {
      return {};
    }
    try {
      final response = await getDioClient().post(
        '/api/pixez/novels/mirror/batch',
        data: {'novel_ids': ids},
      );
      final data = _unwrapData(response);
      if (data is Map<String, dynamic>) {
        final List<dynamic> mirroredIds = data['mirrored_ids'] ?? [];
        return mirroredIds.map((e) => e as int).toSet();
      }
      return {};
    } catch (_) {
      return {};
    }
  }

  static Future<Map<String, dynamic>?> mirrorIllust(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      final response = await getDioClient().post(
        '/api/pixez/illusts/$id/mirror',
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<Response?> getMirroredIllustDetail(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      LPrinter.d(
        '[MirrorDetail] skipped id=$id enabled=${SyncConfig.enabled} '
        'serverUrlConfigured=${SyncConfig.serverUrl.isNotEmpty}',
      );
      return null;
    }
    try {
      LPrinter.d(
        '[MirrorDetail] request id=$id '
        'url=${SyncConfig.serverUrl}/mirror/v1/illust/detail',
      );
      final response = await getDioClient().get(
        '/mirror/v1/illust/detail',
        queryParameters: {'illust_id': id},
      );
      LPrinter.d(
        '[MirrorDetail] response id=$id status=${response.statusCode} '
        'realUri=${response.realUri}',
      );
      return response;
    } catch (error) {
      LPrinter.d('[MirrorDetail] failed id=$id error=$error');
      return null;
    }
  }

  static Future<Map<String, dynamic>?> getNovelMirrorStatus(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      final response = await getDioClient().get('/api/pixez/novels/$id/mirror');
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<bool> isNovelMirrored(int id) async {
    final data = await getNovelMirrorStatus(id);
    return data?['mirrored'] == true;
  }

  static Future<Map<String, dynamic>?> mirrorNovel(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      final response = await getDioClient().post(
        '/api/pixez/novels/$id/mirror',
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<Response?> getMirroredNovelDetail(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      return getDioClient().get(
        '/mirror/v1/novel/detail',
        queryParameters: {'novel_id': id},
      );
    } catch (_) {
      return null;
    }
  }

  static Future<Response?> getMirroredNovelText(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      return getDioClient().get(
        '/mirror/webview/v2/novel',
        queryParameters: {'novel_id': id},
      );
    } catch (_) {
      return null;
    }
  }

  // Mirror management APIs

  static Future<Map<String, dynamic>?> listMirroredIllusts({
    int page = 1,
    int pageSize = 20,
  }) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return null;
    try {
      final response = await getDioClient().get(
        '/api/pixez/mirror/illusts',
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<Map<String, dynamic>?> listMirroredNovels({
    int page = 1,
    int pageSize = 20,
  }) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return null;
    try {
      final response = await getDioClient().get(
        '/api/pixez/mirror/novels',
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      return _unwrapData(response) as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  static Future<bool> deleteMirroredIllust(int illustId) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return false;
    try {
      final response = await getDioClient().delete(
        '/api/pixez/mirror/illusts/$illustId',
      );
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Future<bool> deleteMirroredNovel(int novelId) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return false;
    try {
      final response = await getDioClient().delete(
        '/api/pixez/mirror/novels/$novelId',
      );
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Future<bool> batchDeleteMirrored(
    String targetType,
    List<int> ids,
  ) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) return false;
    try {
      final response = await getDioClient().post(
        '/api/pixez/mirror/batch-delete',
        data: {'target_type': targetType, 'ids': ids},
      );
      return _isSuccess(response);
    } catch (_) {
      return false;
    }
  }

  static Map<String, String> _authHeaders(String accessToken) {
    if (accessToken.isEmpty) {
      return {};
    }
    return {'Authorization': 'Bearer $accessToken'};
  }

  static bool _isSuccess(Response response) {
    final data = response.data;
    return response.statusCode == 200 &&
        data is Map &&
        (data['error_msg'] == null || data['error_msg'] == '');
  }

  static dynamic _unwrapData(Response response) {
    if (!_isSuccess(response)) {
      return null;
    }
    return (response.data as Map)['data'];
  }

  static String _failureMessage(Response response) {
    final data = response.data;
    if (data is Map && data['error_msg'] != null) {
      return data['error_msg'].toString();
    }
    return 'Sync server request failed';
  }
}
