import 'package:dio/dio.dart';
import 'package:pixez/custom/services/sync_config.dart';
import 'package:pixez/custom/services/sync_service.dart';
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
    final data = response?.data;
    if (response?.statusCode == 200 &&
        data is Map &&
        data['illust'] is Map<String, dynamic>) {
      return Illusts.fromJson(data['illust'] as Map<String, dynamic>);
    }
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
}
