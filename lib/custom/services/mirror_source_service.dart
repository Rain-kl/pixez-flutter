import 'package:mobx/mobx.dart';
import 'package:pixez/custom/services/mirror_fallback_service.dart';
import 'package:pixez/page/picture/illust_store.dart';

class MirrorSourceService {
  static Future<bool> loadIllustIntoStore(IllustStore store) async {
    final mirrored = await MirrorFallbackService.getMirroredIllust(store.id);
    if (mirrored == null) {
      return false;
    }
    runInAction(() {
      store.illusts = mirrored;
      store.isBookmark = mirrored.isBookmarked;
      store.state = mirrored.isBookmarked ? 2 : 0;
      store.captionFetchError = false;
      store.errorMessage = null;
    });
    return true;
  }
}
