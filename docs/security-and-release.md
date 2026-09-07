# 安全与发布检查

## 数据与凭据

- SQLite 数据库位于 fnOS 应用私有数据目录，不随仓库提交。
- API Key、下载器密码、飞牛登录会话仅保留在本地数据库/内存中；API 的列表响应会清空密码字段。
- Debug 日志仅记录诊断结果，禁止写入密码、API Key、Cookie、Authorization 头和会话令牌。
- `.gitignore` 忽略构建产物、`.fpk`、数据库、日志、`work/`、`outputs/` 和 `.env`。

## 发布前清单

1. 执行 `git status --ignored`，确认未纳入数据库、构建包和环境文件。
2. 用密钥扫描工具检查历史与待提交内容；人工检查示例地址、账号和截图。
3. 执行 `cd app && go test ./...` 与 `pnpm --dir app/ui build`。
4. 在干净的 fnOS 测试机测试安装、升级、卸载，以及停用链后的原限速恢复。
5. 确认 `CHANGELOG.md`、版本号、安装包校验值和 GitHub Release 内容一致。

构建脚本在交付前检查 FPK 外层/内层 gzip、必需文件、Linux 可执行权限、ELF 架构，并生成 SHA-256。CI 仅在全部测试通过后打包，正式 Release 还要求标签版本匹配且提交已进入 main。Windows SDK 的打包成功提示本身不足以证明 Linux 权限正确；正式包以 Linux 流水线验证结果为准。

不要将真实 NAS 地址、账号、密码、API Key 或会话 Cookie 提交到 issue、截图、日志或 Git 历史中。
