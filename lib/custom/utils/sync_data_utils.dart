import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:pixez/models/ban_comment_persist.dart';
import 'package:pixez/models/ban_illust_id.dart';
import 'package:pixez/models/ban_tag.dart';
import 'package:pixez/models/ban_user_id.dart';
import 'package:pixez/models/illust_persist.dart';
import 'package:pixez/models/novel_persist.dart';
import 'package:pixez/models/tags.dart';

class SyncDataUtils {
  static const tableNames = [
    'ban_comments',
    'ban_illusts',
    'ban_tags',
    'ban_users',
    'illust_histories',
    'novel_histories',
    'tag_histories',
  ];

  static Future<Map<String, String>> computeLocalHashes() async {
    final hashes = <String, String>{};

    try {
      final provider = BanCommenProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['ban_comments'] = computeStableHash(
        list.map((e) => '${e.commentId}:${e.name}').toList(),
      );
    } catch (_) {
      hashes['ban_comments'] = 'empty';
    }

    try {
      final provider = BanIllustIdProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['ban_illusts'] = computeStableHash(
        list.map((e) => '${e.illustId}:${e.name}').toList(),
      );
    } catch (_) {
      hashes['ban_illusts'] = 'empty';
    }

    try {
      final provider = BanTagProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['ban_tags'] = computeStableHash(
        list.map((e) => '${e.name}:${e.translateName}').toList(),
      );
    } catch (_) {
      hashes['ban_tags'] = 'empty';
    }

    try {
      final provider = BanUserIdProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['ban_users'] = computeStableHash(
        list.map((e) => '${e.userId ?? ''}:${e.name ?? ''}').toList(),
      );
    } catch (_) {
      hashes['ban_users'] = 'empty';
    }

    try {
      final provider = IllustPersistProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['illust_histories'] = computeStableHash(
        list.map((e) => '${e.illustId}:${e.userId}:${e.time}').toList(),
      );
    } catch (_) {
      hashes['illust_histories'] = 'empty';
    }

    try {
      final provider = NovelPersistProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['novel_histories'] = computeStableHash(
        list.map((e) => '${e.novelId}:${e.userId}:${e.time}').toList(),
      );
    } catch (_) {
      hashes['novel_histories'] = 'empty';
    }

    try {
      final provider = TagsPersistProvider();
      await provider.open();
      final list = await provider.getAllAccount();
      await provider.close();
      hashes['tag_histories'] = computeStableHash(
        list
            .map((e) => '${e.name}:${e.translatedName}:${e.type ?? 0}')
            .toList(),
      );
    } catch (_) {
      hashes['tag_histories'] = 'empty';
    }

    return hashes;
  }

  static String computeStableHash(List<String> lines) {
    if (lines.isEmpty) return 'empty';
    lines.sort();
    return md5.convert(utf8.encode(lines.join('\n'))).toString();
  }

  static Future<Map<String, dynamic>> collectLocalUserData(
    List<String>? selectTables,
  ) async {
    final payload = <String, dynamic>{};

    if (_shouldInclude(selectTables, 'ban_comments')) {
      final provider = BanCommenProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['ban_comments'] = items.map((e) => e.toJson()).toList();
    }

    if (_shouldInclude(selectTables, 'ban_illusts')) {
      final provider = BanIllustIdProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['ban_illusts'] = items.map((e) => e.toJson()).toList();
    }

    if (_shouldInclude(selectTables, 'ban_tags')) {
      final provider = BanTagProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['ban_tags'] = items.map((e) => e.toJson()).toList();
    }

    if (_shouldInclude(selectTables, 'ban_users')) {
      final provider = BanUserIdProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['ban_users'] = items.map((e) => e.toJson()).toList();
    }

    if (_shouldInclude(selectTables, 'illust_histories')) {
      final provider = IllustPersistProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['illust_histories'] = items.map((e) => e.toJson()).toList();
    }

    if (_shouldInclude(selectTables, 'novel_histories')) {
      final provider = NovelPersistProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['novel_histories'] = items.map((e) => e.toJson()).toList();
    }

    if (_shouldInclude(selectTables, 'tag_histories')) {
      final provider = TagsPersistProvider();
      await provider.open();
      final items = await provider.getAllAccount();
      await provider.close();
      payload['tag_histories'] = items.map((e) => e.toJson()).toList();
    }

    return payload;
  }

  static Future<void> restoreUserData(Map<String, dynamic> data) async {
    if (data.containsKey('ban_comments') && data['ban_comments'] != null) {
      final items = (data['ban_comments'] as List? ?? [])
          .map((e) => BanCommentPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = BanCommenProvider();
      await provider.open();
      await provider.deleteAll();
      for (final item in items) {
        await provider.insert(item);
      }
      await provider.close();
    }

    if (data.containsKey('ban_illusts') && data['ban_illusts'] != null) {
      final items = (data['ban_illusts'] as List? ?? [])
          .map((e) => BanIllustIdPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = BanIllustIdProvider();
      await provider.open();
      await provider.deleteAll();
      if (items.isNotEmpty) {
        await provider.insertAll(items);
      }
      await provider.close();
    }

    if (data.containsKey('ban_tags') && data['ban_tags'] != null) {
      final items = (data['ban_tags'] as List? ?? [])
          .map((e) => BanTagPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = BanTagProvider();
      await provider.open();
      await provider.deleteAll();
      if (items.isNotEmpty) {
        await provider.insertAll(items);
      }
      await provider.close();
    }

    if (data.containsKey('ban_users') && data['ban_users'] != null) {
      final items = (data['ban_users'] as List? ?? [])
          .map((e) => BanUserIdPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = BanUserIdProvider();
      await provider.open();
      await provider.deleteAll();
      if (items.isNotEmpty) {
        await provider.insertAll(items);
      }
      await provider.close();
    }

    if (data.containsKey('illust_histories') &&
        data['illust_histories'] != null) {
      final items = (data['illust_histories'] as List? ?? [])
          .map((e) => IllustPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = IllustPersistProvider();
      await provider.open();
      await provider.deleteAll();
      for (final item in items) {
        await provider.insert(item);
      }
      await provider.close();
    }

    if (data.containsKey('novel_histories') &&
        data['novel_histories'] != null) {
      final items = (data['novel_histories'] as List? ?? [])
          .map((e) => NovelPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = NovelPersistProvider();
      await provider.open();
      await provider.deleteAll();
      for (final item in items) {
        await provider.insert(item);
      }
      await provider.close();
    }

    if (data.containsKey('tag_histories') && data['tag_histories'] != null) {
      final items = (data['tag_histories'] as List? ?? [])
          .map((e) => TagsPersist.fromJson(e as Map<String, dynamic>))
          .toList();
      final provider = TagsPersistProvider();
      await provider.open();
      await provider.deleteAll(type: 0);
      await provider.deleteAll(type: 1);
      if (items.isNotEmpty) {
        await provider.insertAll(items);
      }
      await provider.close();
    }
  }

  static bool _shouldInclude(List<String>? selectTables, String table) {
    return selectTables == null || selectTables.contains(table);
  }
}
