import 'package:dio/dio.dart';
import 'package:pixez/custom/services/sync_api.dart';
import 'package:pixez/lighting/lighting_store.dart';

class RemovedBookmarkSource extends ApiForceSource implements PagedLightSource {
  RemovedBookmarkSource({required int userId})
      : super(
          futureGet: (bool force) =>
              SyncApi.getRemovedBookmarkIllusts(userId: userId),
        );

  @override
  Future<Response> fetchNext(String url) {
    return SyncApi.getRemovedBookmarkIllustsNext(url);
  }
}
