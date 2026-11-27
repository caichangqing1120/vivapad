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

# Windows / Linux（在对应平台或 CI Runner 执行）
GOWORK=off go tool viva package --target windows --arch amd64 --no-sign
GOWORK=off go tool viva package --target linux --arch amd64 --no-sign
```

## 本地私有文件红线

严禁将 `docs/`、`specs/`、`plans/`、`screenshots/`、`captures/`、`plan*.md`、`spec*.md` 提交至 Git 或上传至 Issue/PR 附件。`tools/github/check-public-files.cjs` 会在提交前与 CI 中强制拦截。
