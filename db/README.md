# 数据库

应用使用 SQLite 保存配置与运行状态。程序启动时会自动创建或升级表结构，不需要手工导入 SQL。

## 存储位置

- fnOS 安装环境：生命周期脚本通过 `TRIM_PKGVAR` 指向应用私有数据目录。
- 本地开发：默认使用本目录下的 `runtime/throttle.db`。
- 自定义位置：设置 `DATA_DIR` 环境变量。

`runtime/`、`*.db`、WAL 和 SHM 文件均被 Git 忽略，不得提交到 GitHub，因为数据库可能包含下载器密码、API Key 和本地服务地址。

## 主要数据表

| 表 | 用途 |
| --- | --- |
| `libraries` | 影视库连接、轮询设置和最近观看人数 |
| `downloaders` | 下载器连接、原始限速和当前状态 |
| `binding_chains` | 绑定链与上传/下载限速规则 |
| `chain_libraries` | 绑定链与影视库的多对多关系 |
| `chain_downloaders` | 绑定链与下载器的多对多关系 |
| `events` | 运行、连接与限速变更日志 |
| `settings` | 应用级设置 |
| `release_limits` | 最后启用规则的基础上限（bytes/s），绑定删除后用于最高限速释放策略 |

连接入口位于 `app/storage/database.go`，权威 schema 位于 `app/storage/migrations.go`；查询按资源拆分在同目录。文档不复制可执行 SQL，以避免结构说明和实际迁移失去同步。

每个 Store 使用一条连接、WAL、5 秒 busy_timeout 和立即事务。增加查询时必须先关闭上一组 rows，再调用其他存储方法；事务内复用 tx，不能改用 Store.DB 再借连接。

旧 `bindings` 表只用于启动时的一次性升级，不再提供旧 CRUD/API。全部旧关系在一个事务中转换；成功后删除旧表，再清理下载器上的旧规则字段。失败回滚并保留旧数据，下次启动可重试；已转换的链保持原配置，尚未转换的关系继续迁移。升级前仍建议备份整个应用数据目录。

`settings.release_mode` 的值为 `original`（默认）、`unlimited` 或 `maximum`。`release_limits` 不随绑定删除，随下载器删除级联清理。旧库升级会为当前启用绑定链回填快照；若历史绑定已删除且没有快照，最高限速释放将报错并保留接管记录，用户可改选恢复原值重试。
