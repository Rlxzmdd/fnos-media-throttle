# 扩展适配器

## 新影视库

在 `app/media/` 中实现 `media.Adapter`：

```go
type Adapter interface {
    Test(context.Context, Library) error
    ActiveViewerCount(context.Context, Library) (int, error)
}
```

`ActiveViewerCount` 应返回当前活跃的**播放终端数**。可用“账号 + 设备”作身份键；缺设备标识时回退为账号。实现必须尊重 `Library.CountLocalClients`：有客户端 IP/本地标志时可过滤；没有足够数据时返回清晰能力错误，不能把无法判定的数据伪装成零人。

在 `app/service/engine.go` 的 `media.NewRegistry` 调用中添加 `media.Registration{Kind: 新类型, Impl: 实现{}}`，并补充模拟响应测试。接口声明、注册表和具体实现应分文件保存。

## 新下载器

在 `app/downloader/` 中实现 `downloader.Adapter`：

```go
type Adapter interface {
    Test(context.Context, Downloader) error
    Limits(context.Context, Downloader) (upload, download int64, err error)
    Stats(context.Context, Downloader) (TransferStats, error)
    ApplyLimits(context.Context, Downloader, LimitPatch) error
}
```

`LimitPatch` 的 `nil` 表示该方向不写入。这一约定使上传和下载可以独立接管/恢复。适配器不得记录密码、令牌或完整认证头；注册到 `downloader.Registry` 后由 Service 统一处理原值保存和恢复。接口声明、注册表和具体实现应分文件保存。

在 `app/service/engine.go` 中注册，例如：

```go
downloaders := downloader.NewRegistry(
    downloader.Registration{Kind: domain.DownloaderQB, Impl: downloader.QBittorrentAdapter{}},
    // 在此添加新类型；实际应用还需保留其余已有提供者。
)
```

注册表不对未知类型进行协议猜测；未注册类型返回错误。`Engine.Downloaders` 是测试连接和限速的唯一注入入口，旧 `QBit` 字段已删除。新增类型还需补充 `domain` 常量、`api/validation.go` 校验、UI 类型选项及默认表单；仅注册后端不等于完成 UI 接入。

最低测试集：成功/失败认证、上下行分别修改、nil 不修改、0 不限速、超时取消、原值恢复。所有网络方法都应尊重 context；`Stats` 中速度和上限都使用 bytes/s。使用 fake/httptest，不在测试中内置真实 NAS 凭据。

需要在一次顺序轮询内复用认证连接时，可额外实现 `downloader.CycleAdapter.BeginCycle`，返回周期上下文和清理函数。注册表自动委托，Service 负责 defer 清理；无需在业务服务中新增厂商专用分支。普通 HTTP 适配器无需实现该可选接口。

## 新写操作与并发约定

API 写操作统一经过 `api/commands.go`，必须使用请求传入的 context。非 HTTP 的配置变更也应由 `Engine.Coordinate` 包裹整个“读取配置 → 修改配置 → 写入/释放设备”过程。不要只锁最后一次写入，更不要直接从异步任务调用适配器绕过 Engine。

协调区 context 只可用于同步嵌套调用，不得传给新 goroutine。`StopCommands` 后拒绝新写命令，只有 `Shutdown` 能使用独立恢复上下文执行最终释放。增加行为时补充 `api/concurrency_test.go` 一类可控交错测试；测试不能依赖 sleep 猜测顺序。
