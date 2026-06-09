import 'package:dio/dio.dart';
import 'package:pixez/custom/services/sync_api.dart';
import 'package:pixez/custom/services/sync_scheduler_service.dart';
import 'package:pixez/custom/services/sync_user_data_service.dart';
import 'package:pixez/models/account.dart';

class SyncService {
  static Future<bool> ping({String? url, String? accessToken}) {
    return SyncApi.ping(url: url, accessToken: accessToken);
  }

  static Future<List<dynamic>> listUsers() {
    return SyncApi.listUsers();
  }

  static Future<Map<String, dynamic>?> getUser(String pixivUserId) {
    return SyncApi.getUser(pixivUserId);
  }

  static Future<bool> upsertUser(AccountPersist account) {
    return SyncApi.upsertUser(account);
  }

  static Future<bool> deleteUser(String pixivUserId) {
    return SyncApi.deleteUser(pixivUserId);
  }

  static Future<Map<String, dynamic>?> getIllustMirrorStatus(int id) {
    return SyncApi.getIllustMirrorStatus(id);
  }

  static Future<bool> isIllustMirrored(int id) {
    return SyncApi.isIllustMirrored(id);
  }

  static Future<Set<int>> batchCheckIllustMirror(List<int> ids) {
    return SyncApi.batchCheckIllustMirror(ids);
  }

  static Future<Set<int>> batchCheckNovelMirror(List<int> ids) {
    return SyncApi.batchCheckNovelMirror(ids);
  }

  static Future<Map<String, dynamic>?> mirrorIllust(int id) {
    return SyncApi.mirrorIllust(id);
  }

  static Future<Response?> getMirroredIllustDetail(int id) {
    return SyncApi.getMirroredIllustDetail(id);
  }

  static Future<Map<String, dynamic>?> getNovelMirrorStatus(int id) {
    return SyncApi.getNovelMirrorStatus(id);
  }

  static Future<bool> isNovelMirrored(int id) {
    return SyncApi.isNovelMirrored(id);
  }

  static Future<Map<String, dynamic>?> mirrorNovel(int id) {
    return SyncApi.mirrorNovel(id);
  }

  static Future<Response?> getMirroredNovelDetail(int id) {
    return SyncApi.getMirroredNovelDetail(id);
  }

  static Future<Response?> getMirroredNovelText(int id) {
    return SyncApi.getMirroredNovelText(id);
  }

  static Future<Map<String, String>?> fetchRemoteHashes(String userId) {
    return SyncUserDataService.fetchRemoteHashes(userId);
  }

  static Future<Map<String, String>> computeLocalHashes() {
    return SyncUserDataService.computeLocalHashes();
  }

  static void startPeriodicSyncTimer() {
    SyncSchedulerService.startPeriodicSyncTimer();
  }

  static void stopPeriodicSyncTimer() {
    SyncSchedulerService.stopPeriodicSyncTimer();
  }

  static Future<void> syncData(String userId) {
    return SyncUserDataService.syncData(userId);
  }

  static Future<List<String>?> backupUserDataSelective(String userId) {
    return SyncUserDataService.backupUserDataSelective(userId);
  }

  static Future<List<String>?> restoreUserDataSelective(String userId) {
    return SyncUserDataService.restoreUserDataSelective(userId);
  }

  static Future<bool> uploadUserData(String userId) {
    return SyncUserDataService.uploadUserData(userId);
  }

  static Future<bool> uploadSelectiveUserData(
    String userId,
    List<String>? selectTables,
  ) {
    return SyncUserDataService.uploadSelectiveUserData(userId, selectTables);
  }

  static Future<bool> downloadAndRestoreUserData(String userId) {
    return SyncUserDataService.downloadAndRestoreUserData(userId);
  }

  static Future<bool> downloadAndRestoreSelectiveUserData(
    String userId,
    List<String>? selectTables,
  ) {
    return SyncUserDataService.downloadAndRestoreSelectiveUserData(
      userId,
      selectTables,
    );
  }
}
