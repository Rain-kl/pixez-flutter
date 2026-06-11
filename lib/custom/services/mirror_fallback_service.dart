import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';
import 'package:pixez/er/lprinter.dart';
import 'package:pixez/models/illust.dart';

class MirrorFallbackService {
  static const String limitUnknownImage = 'limit_unknown_360.png';

  static bool get enabled =>
      SyncConfig.enabled && SyncConfig.serverUrl.isNotEmpty;

  static Future<Illusts?> getMirroredIllust(int id) async {
    if (!enabled) {
      return null;
    }
    final Response? response = await SyncService.getMirroredIllustDetail(id);
    final data = _decodeMap(response?.data);
    if (response?.statusCode == 200 && data['illust'] is Map<String, dynamic>) {
      final illust = Illusts.fromJson(data['illust'] as Map<String, dynamic>);
      LPrinter.d(
        '[MirrorDetail] parsed id=$id '
        'squareMedium=${illust.imageUrls.squareMedium} '
        'medium=${illust.imageUrls.medium} '
        'large=${illust.imageUrls.large} '
        'original=${illust.metaSinglePage?.originalImageUrl}',
      );
      return illust;
    }
    LPrinter.d(
      '[MirrorDetail] invalid response id=$id status=${response?.statusCode} '
      'hasIllust=${data['illust'] is Map<String, dynamic>}',
    );
    return null;
  }

  static String? mirrorImageUrl(String url) {
    if (!enabled) {
      return null;
    }
    final uri = Uri.tryParse(url);
    if (uri == null ||
        (uri.host != 'i.pximg.net' && uri.host != 's.pximg.net')) {
      return null;
    }
    return '${SyncConfig.serverUrl}/mirror/pximg${uri.path}';
  }

  static bool isMirrorImageUrl(String? url) {
    if (url == null || !enabled) {
      return false;
    }
    final uri = Uri.tryParse(url);
    final serverUri = Uri.tryParse(SyncConfig.serverUrl);
    if (uri == null || serverUri == null) {
      return false;
    }
    return uri.scheme == serverUri.scheme &&
        uri.host == serverUri.host &&
        uri.port == serverUri.port &&
        uri.path.startsWith('/mirror/pximg/');
  }

  static bool isLimitUnknownIllust(Illusts illust) {
    return _isLimitUnknownUrl(illust.imageUrls.squareMedium) ||
        _isLimitUnknownUrl(illust.imageUrls.medium) ||
        _isLimitUnknownUrl(illust.imageUrls.large) ||
        _isLimitUnknownUrl(illust.metaSinglePage?.originalImageUrl) ||
        illust.metaPages.any((page) {
          final imageUrls = page.imageUrls;
          return imageUrls != null &&
              (_isLimitUnknownUrl(imageUrls.squareMedium) ||
                  _isLimitUnknownUrl(imageUrls.medium) ||
                  _isLimitUnknownUrl(imageUrls.large) ||
                  _isLimitUnknownUrl(imageUrls.original));
        });
  }

  static bool _isLimitUnknownUrl(String? url) {
    return url != null && url.contains(limitUnknownImage);
  }

  static Map<String, dynamic> _decodeMap(dynamic data) {
    if (data is Map<String, dynamic>) {
      return data;
    }
    if (data is String) {
      final decoded = jsonDecode(data);
      if (decoded is Map<String, dynamic>) {
        return decoded;
      }
    }
    return const {};
  }
}
