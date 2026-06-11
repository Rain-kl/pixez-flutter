import 'dart:typed_data';

import 'package:file/file.dart' as pfile;
import 'package:flutter_cache_manager/flutter_cache_manager.dart';
import 'package:flutter_cache_manager_dio/flutter_cache_manager_dio.dart';
import 'package:pixez/custom/services/mirror_fallback_service.dart';

class PixivCacheManager implements BaseCacheManager {
  PixivCacheManager._();

  static final PixivCacheManager instance = PixivCacheManager._();

  // The rhttp-based cache manager using Dio.
  BaseCacheManager get _rhttpManager => DioCacheManager.instance;

  // The default cache manager using standard Dart HttpClient (which uses system TLS).
  late final BaseCacheManager _defaultManager = CacheManager(
    Config('pixezMirrorImageCache'),
  );

  BaseCacheManager _managerFor(String urlOrKey) {
    if (MirrorFallbackService.isMirrorImageUrl(urlOrKey)) {
      return _defaultManager;
    }
    return _rhttpManager;
  }

  @override
  Future<FileInfo> downloadFile(
    String url, {
    String? key,
    Map<String, String>? authHeaders,
    bool force = false,
  }) {
    return _managerFor(url).downloadFile(
      url,
      key: key,
      authHeaders: authHeaders,
      force: force,
    );
  }

  @override
  Future<FileInfo?> getFileFromCache(
    String key, {
    bool ignoreMemCache = false,
  }) {
    return _managerFor(key).getFileFromCache(
      key,
      ignoreMemCache: ignoreMemCache,
    );
  }

  @override
  Future<FileInfo?> getFileFromMemory(String key) {
    return _managerFor(key).getFileFromMemory(key);
  }

  @override
  Stream<FileResponse> getFileStream(
    String url, {
    String? key,
    Map<String, String>? headers,
    bool withProgress = false,
  }) {
    return _managerFor(url).getFileStream(
      url,
      key: key,
      headers: headers,
      withProgress: withProgress,
    );
  }

  @override
  Future<pfile.File> putFile(
    String url,
    Uint8List fileBytes, {
    String? key,
    String? eTag,
    Duration maxAge = const Duration(days: 30),
    String fileExtension = 'file',
  }) {
    return _managerFor(url).putFile(
      url,
      fileBytes,
      key: key,
      eTag: eTag,
      maxAge: maxAge,
      fileExtension: fileExtension,
    );
  }

  @override
  Future<pfile.File> putFileStream(
    String url,
    Stream<List<int>> source, {
    String? key,
    String? eTag,
    Duration maxAge = const Duration(days: 30),
    String fileExtension = 'file',
  }) {
    return _managerFor(url).putFileStream(
      url,
      source,
      key: key,
      eTag: eTag,
      maxAge: maxAge,
      fileExtension: fileExtension,
    );
  }

  @override
  Future<void> removeFile(String key) {
    return _managerFor(key).removeFile(key);
  }

  @override
  Future<void> emptyCache() async {
    await Future.wait([
      _defaultManager.emptyCache(),
      _rhttpManager.emptyCache(),
    ]);
  }

  @override
  Future<void> dispose() async {
    await Future.wait([
      _defaultManager.dispose(),
      _rhttpManager.dispose(),
    ]);
  }

  @override
  Stream<FileInfo> getFile(
    String url, {
    Map<String, String>? headers,
    String? key,
  }) {
    return _managerFor(url).getFile(
      url,
      headers: headers ?? const {},
      key: key ?? url,
    );
  }

  @override
  Future<pfile.File> getSingleFile(
    String url, {
    String? key,
    Map<String, String>? headers,
  }) {
    return _managerFor(url).getSingleFile(
      url,
      key: key ?? url,
      headers: headers ?? const {},
    );
  }
}
