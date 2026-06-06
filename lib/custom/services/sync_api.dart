import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:pixez/custom/services/sync_config.dart';
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

    dio.options.headers['Authorization'] = _basicAuth(
      SyncConfig.username,
      SyncConfig.password,
    );
    return dio;
  }

  static Future<bool> ping({
    String? url,
    String? username,
    String? password,
  }) async {
    final targetUrl = url ?? SyncConfig.serverUrl;
    final targetUser = username ?? SyncConfig.username;
    final targetPass = password ?? SyncConfig.password;

    if (targetUrl.isEmpty || targetUser.isEmpty || targetPass.isEmpty) {
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
      dio.options.headers['Authorization'] = _basicAuth(targetUser, targetPass);

      final response = await dio.get('/api/pixez/ping');
      return response.statusCode == 200 && response.data?['success'] == true;
    } catch (_) {
      return false;
    }
  }

  static Future<List<dynamic>> listUsers() async {
    try {
      final response = await getDioClient().get('/api/pixez/users');
      if (response.statusCode == 200 && response.data?['success'] == true) {
        return response.data['data'] as List<dynamic>;
      }
      return [];
    } catch (_) {
      return [];
    }
  }

  static Future<Map<String, dynamic>?> getUser(String pixivUserId) async {
    try {
      final response = await getDioClient().get(
        '/api/pixez/users/$pixivUserId',
      );
      if (response.statusCode == 200 && response.data?['success'] == true) {
        return response.data['data'] as Map<String, dynamic>;
      }
      return null;
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
      return response.statusCode == 200 && response.data?['success'] == true;
    } catch (_) {
      return false;
    }
  }

  static Future<bool> deleteUser(String pixivUserId) async {
    try {
      final response = await getDioClient().delete(
        '/api/pixez/users/$pixivUserId',
      );
      return response.statusCode == 200 && response.data?['success'] == true;
    } catch (_) {
      return false;
    }
  }

  static Future<Map<String, String>?> fetchRemoteHashes(String userId) async {
    try {
      final response = await getDioClient().get(
        '/api/pixez/users/$userId/sync-data/hashes',
      );
      if (response.statusCode == 200 && response.data?['success'] == true) {
        final data = response.data['data'] as Map<String, dynamic>;
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
      return response.statusCode == 200 && response.data?['success'] == true;
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
      if (response.statusCode != 200 ||
          response.data == null ||
          response.data['success'] != true) {
        return null;
      }
      return response.data['data'] as Map<String, dynamic>;
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
      if (response.statusCode == 200 && response.data?['success'] == true) {
        return response.data['data'] as Map<String, dynamic>;
      }
      return null;
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
    if (response.statusCode == 200 && response.data?['success'] == true) {
      return Response(
        requestOptions: response.requestOptions,
        data: response.data['data'],
        statusCode: response.statusCode,
        statusMessage: response.statusMessage,
        headers: response.headers,
        redirects: response.redirects,
        isRedirect: response.isRedirect,
        extra: response.extra,
      );
    }
    throw Exception(response.data?['message'] ?? 'Sync server request failed');
  }

  static Future<bool> isIllustMirrored(int id) async {
    final data = await getIllustMirrorStatus(id);
    return data?['mirrored'] == true;
  }

  static Future<Map<String, dynamic>?> mirrorIllust(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      final response = await getDioClient().post(
        '/api/pixez/illusts/$id/mirror',
      );
      if (response.statusCode == 200 && response.data?['success'] == true) {
        return response.data['data'] as Map<String, dynamic>;
      }
      return null;
    } catch (_) {
      return null;
    }
  }

  static Future<Response?> getMirroredIllustDetail(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      return getDioClient().get(
        '/mirror/v1/illust/detail',
        queryParameters: {'illust_id': id},
      );
    } catch (_) {
      return null;
    }
  }

  static Future<Map<String, dynamic>?> getNovelMirrorStatus(int id) async {
    if (!SyncConfig.enabled || SyncConfig.serverUrl.isEmpty) {
      return null;
    }
    try {
      final response = await getDioClient().get(
        '/api/pixez/novels/$id/mirror',
      );
      if (response.statusCode == 200 && response.data?['success'] == true) {
        return response.data['data'] as Map<String, dynamic>;
      }
      return null;
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
      if (response.statusCode == 200 && response.data?['success'] == true) {
        return response.data['data'] as Map<String, dynamic>;
      }
      return null;
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


  static String _basicAuth(String username, String password) {
    return 'Basic ${base64Encode(utf8.encode('$username:$password'))}';
  }
}
