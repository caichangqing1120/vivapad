# Contributing To VivaPad

VivaPad 是一个原生支持 **Windows、Linux、macOS** 的跨平台编辑器，采用**里程碑批次驱动（Milestone-Batch）**的敏捷开发流程。

## 开发与提交流程

1. **按里程碑批次推进**：围绕本地 `docs/specs/001-vivapad/plan.md` 中的单个里程碑（如 `M1`、`M2`、`M3`、`M4`）开展工作，每个功能必须保证 Windows、Linux、macOS 三端代码与构建完整可用，不在日常开发中将微小修复拆成数十个碎片 Issue。
2. **远端 PR 与三端 CI 验收**：
   - 每个里程碑创建 **1 个总括 Issue（Umbrella Issue）**，评论 `/claim` 获取机器人分配确认后，在该里程碑专属分支（如 `codex/m1-cross-platform-themes`）内连续完成全部子项。
   - 收口时提交 1 个汇总 PR（正文包含 `Closes #123`），由 GitHub Actions 原生三端矩阵（Windows / Linux / macOS）执行自动化测试与三端构建校验。
3. **Viva 框架边界**：保持 `go.mod` 中已固定的 Viva 版本稳定；除非阻塞核心跨平台功能且获用户明确同意，不在日常任务中频繁升级 Viva 依赖。

## 验证分级

### 日常子任务验证（秒级）

```sh
GOWORK=off go test ./internal/<pkg> ./cmd/vivapad -run '<FocusedTest>' -count=1
GOOS=windows GOARCH=amd64 GOWORK=off go vet ./internal/<pkg> ./cmd/vivapad
GOOS=linux GOARCH=amd64 GOWORK=off go vet ./internal/<pkg> ./cmd/vivapad
```

### 里程碑收口门禁（每个里程碑跑一次）

```sh
test -z "$(gofmt -l $(git ls-files '*.go'))"
GOWORK=off go vet ./...
GOWORK=off go test ./... -count=1
GOOS=linux GOARCH=amd64 GOWORK=off go build ./cmd/vivapad
GOOS=windows GOARCH=amd64 GOWORK=off go build ./cmd/vivapad
node tools/github/check-public-files.cjs
```

三端打包命令参考：

```sh
# macOS (arm64)
GOWORK=off go tool viva package --target darwin --arch arm64 --format app --no-sign -o build/VivaPad.app
GOWORK=off go run ./tools/icon -app build/VivaPad.app

# Windows（需要 makensis）/ Linux
GOWORK=off go tool viva package --target windows --arch amd64 --format nsis,zip --no-sign -o build/packages
GOWORK=off go tool viva package --target linux --arch amd64 --format deb,tar.gz --no-sign -o build/packages
```

原生三端 CI 通过后，在对应任务的 `vivapad-packages-*` artifact 中提供未签名桌面包。

## 签名更新发布

- `.github/workflows/release.yml` 仅处理稳定版 `vX.Y.Z` tag。先构建、测试和打包，再与上一已发布稳定版的同平台压缩包生成增量补丁；补丁超过全量包的 60% 就不发布。应用内只信任该版本编入的公开 Feed 地址和 Ed25519 公钥。普通本地构建不联网检查，启动检查默认关闭。
- 首次发布前，在可信环境运行 `GOWORK=off go tool viva update-keygen`。把私钥设为仓库 secret `VIVA_UPDATE_KEY`，公钥设为仓库变量 `VIVAPAD_UPDATE_PUBLIC_KEY`。不要提交或打印私钥；换钥前要考虑已经安装的版本仍只信任旧钥。
- macOS 公开发布需要 `APPLE_SIGNING_IDENTITY`、`APPLE_API_KEY_ID`、`APPLE_API_ISSUER` 仓库变量，以及 `APPLE_CERTIFICATE`（base64 `.p12`）、`APPLE_CERTIFICATE_PASSWORD`、`APPLE_API_KEY_P8`（`.p8` 文本内容）secret。Windows 可配置 `WINDOWS_CERTIFICATE`（base64 `.pfx`）和 `WINDOWS_CERTIFICATE_PASSWORD`。没有这些 Windows 凭证时，Feed 仍要求 Ed25519 签名，但 Windows 可能显示 SmartScreen 提示。
- 本地可运行 `bash scripts/build-update-feed.sh RELEASE_DIR 1.2.3 HTTPS_BASE_URL [OS/ARCH=OLD_VERSION=ARCHIVE ...]` 验证签名、补丁生成和 `SHA256SUMS`，不会上传或发布。Linux `.deb` 由系统包管理器更新；可写目录里的归档安装可在应用内更新。

## 本地私有文件红线

严禁将 `docs/`、`specs/`、`plans/`、`screenshots/`、`captures/`、`plan*.md`、`spec*.md` 提交至 Git 或上传至 Issue/PR 附件。`tools/github/check-public-files.cjs` 会在提交前与 CI 中强制拦截。
