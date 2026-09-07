# 目录结构

```text
app/                          应用本体
  main.go                     进程启动、数据目录与 HTTP 服务装配
  api/                        /api/v1 路由与资源处理器
  domain/                     影视库、下载器、绑定链、设置与事件模型
  media/                      影视库适配器接口、注册表与具体实现
    interface.go              稳定影视库适配器接口
    registry.go               kind 到适配器的路由
    http.go                   Emby、Jellyfin 与通用 HTTP 请求实现
    sessions.go              Emby/Jellyfin 播放会话计数
    proxy_response.go         通用 HTTP 人数响应解析
    fnos.go                   飞牛影视 SQLite 实现
  downloader/                 下载器适配器接口、注册表与具体实现
    interface.go              稳定下载器适配器接口
    registry.go               kind 到适配器的路由
    qbittorrent.go            qBittorrent 实现
    transmission.go           Transmission 实现
    fnos_adapter.go           飞牛下载适配器入口
    fnos_gateway.go           飞牛 WebSocket 连接与请求生命周期
    fnos_auth.go              飞牛认证与业务用户选择
    fnos_frame.go             WebSocket 帧编解码
    fnos_crypto.go            登录协议加密辅助
    fnos_values.go            飞牛响应值解析
  chain/                      绑定链规则纯计算
  storage/                    SQLite schema、迁移与分资源持久化
    database.go               连接打开、关闭及初始化失败处理
    migrations.go             schema 与兼容字段升级
    settings.go               设置读取、校验与持久化
    release_limits.go         删除绑定后仍保留的最高限速快照
    values.go                 数据库值转换
    legacy_migration.go       旧版绑定关系到绑定链的一次性迁移
  service/                    轮询、限速、恢复和调度编排
    coordinator.go            API 写命令、轮询与释放的统一协调/取消入口
    release.go                自动退出策略与按方向释放
    restore.go                手动恢复原值与共享释放写入流程
    shutdown.go               有时限的批量退出收尾
  diagnostics/                可选 Debug 日志上下文
  ui/                         Vue 单页管理界面
    src/api.js                HTTP 客户端与错误转换
    src/forms.js              独立表单默认值
    src/icons.js              SVG 图标集
    src/log-format.js         历史日志速度单位的只读显示转换
    src/components/SettingsPanel.js  设置视图（通过事件提交，不直接持久化）
    src/components/ChainLimits.js    绑定链上下行规则摘要
    test/                     Node 测试与组件服务端渲染检查
scripts/                      构建、检查与打包脚本
db/                           数据库说明与本地运行数据
  runtime/                    本地数据库（Git 忽略）
packaging/                    fnOS manifest、生命周期脚本、图标和 UI 配置
docs/                         架构、扩展、安全和发布文档
outputs/                      本地发布产物（Git 忽略）
.github/workflows/ci.yml      测试、竞态检测、双架构 FPK 构建及标签发布
CHANGELOG.md                  版本变更记录
SECURITY.md                   安全问题报告方式
```

依赖方向固定为：`api/main` → `service/storage` → `media/downloader/chain/domain`；适配器、规则和存储依赖 `domain`，不反向引用服务。具体服务协议只能放在对应适配器包内，API、绑定链和轮询服务不直接依赖某个厂商实现。

影视库和下载器的接口、注册表、各厂商实现分别放在同一功能目录中，但接口与实现分文件保存。新增服务时实现对应 `Adapter` 并在 Registry 注册；不要把服务专属认证、协议解析或探测逻辑写入 Engine、HTTP 路由或领域模型。

仓库根目录只保留应用、脚本、数据库说明、fnOS 打包资源、文档和发布元数据。`dist/`、`outputs/`、`packaging/app/`、`app/ui/dist/`、`app/ui/node_modules/` 与 `db/runtime/` 均为本地生成内容，不属于源码发布内容。
