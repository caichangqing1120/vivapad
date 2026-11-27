# VivaPad Agent Instructions (Cross-Platform Pragmatic Mode)

## 1. 核心定位与最高准则

- **唯一北极星目标**：交付在 **Windows、Linux、macOS 三大桌面平台**均可作为日常主力直接替代 Notepad++ 的轻量、极速原生编辑器（大文件秒开、护眼暗色主题、JSON/XML/Diff 常用生产力工具、稳定查找替换与列编辑）。
- **权威计划入口**：始终以本地 `docs/specs/001-vivapad/plan.md` 为唯一当前执行计划（历史归档位于 `docs/specs/001-vivapad/archive/`，仅供查阅，不再续写）。
- **本地私有边界**：`docs/`、Spec、计划、截图均属本地私有材料，严禁提交到 Git 或上传至 Issue/PR 附件。提交前运行 `node tools/github/check-public-files.cjs`。

## 2. 跨平台工程准则（Windows + Linux + macOS 一等公民）

- **一次编写，三端原生可用**：
  - 界面与交互优先构建在 Viva 跨平台 `ui` / `app` / `components` 之上，严禁硬编码仅单平台生效的逻辑；
  - 涉及底层 OS 能力时（如大文件 `mmap`/文件分页读取、系统深浅色检测、文件属性、外部打开），**必须同步提供 `_unix.go`（macOS/Linux）与 `_windows.go` 三端完整实现与单测**，严禁写死 `/` 路径分隔符或缺少 Windows/Linux 分支导致跨平台不可用；
  - 快捷键统一使用 `Primary` 修饰键抽象（macOS 自动映射 `Cmd`，Windows/Linux 自动映射 `Ctrl`）。
- **高效的三端验证分工（拒绝本机套娃虚拟机空转）**：
  - **本地开发时**：跑当前平台聚焦单测 + 秒级三端编译/静态检查（`GOOS=windows GOARCH=amd64` 与 `GOOS=linux GOARCH=amd64` `go build` / `go vet`）；
  - **里程碑收口时**：通过 GitHub Actions 原生三端矩阵（`windows-latest`、`ubuntu-24.04`、`macos-latest`）执行真实三端 `go test` 与三端桌面包构建（Windows `.exe`/安装包、Linux `.deb`/二进制、macOS `.app`）；
  - **严禁**因为当前宿主机是 macOS 没有连接物理 Windows 显示器就拒绝推进下一里程碑，也**严禁**在本机 `/private/tmp` 反复克隆数十 GB 的一次性 Linux rootfs 镜像。

## 3. 极速批次执行规范（废除微粒度拆单空转）

- **按里程碑批次（Milestone Batch）开发**：
  - 围绕 `plan.md` 中的单个里程碑（`M1`–`M4`），在**单一分支/工作树**中连续完成全部子项（5–10 个功能点整批推进），**严禁每改几行代码就新建分支、新建 worktree 或拆成碎片 Issue/PR**。
  - 向 GitHub 推送 PR 时，**每个里程碑只建 1 个总括 Umbrella Issue**（认领 `/claim` 一次覆盖整批任务），里程碑收口时提 **1 个汇总 PR** 触发三端 CI 验收。
- **分层验证**：
  - 日常每个小点只跑对应包聚焦测试（`< 5 秒`）；整个里程碑完成后统一跑全量 `go test ./...`、三端交叉构建与桌面打包。
- **缓存纪律**：复用系统共享 `GOCACHE` / `GOMODCACHE`，禁止创建独立 `HOME`/编译缓存副本，临时测试二进制用完即删。

## 4. 明确冻结项（Hard Stop List — 未经用户点名严禁触碰）

1. **禁止源码考古与像素死磕**：不再编写 Scintilla C++ Oracle，不再要求与原版 Notepad++ 逐像素比对一致，不再为 640×400 极端小窗微调布局。三端原生美观顺手即为通过。
2. **禁止冷门特性扩编**：不再新增冷门上古语言高亮（如 MMIXAL/BaanC/Gui4Cli 等）或罕见 OEM 代码页。
3. **禁止频繁追更 Viva 依赖**：保持当前 `go.mod` 中已验证的 Viva 版本稳定，仅在里程碑边界或确有跨平台阻塞缺陷时按批升级，严禁逐 commit 发 PR 升依赖。
4. **未经用户明确指令，不得擅自 push、merge 或清理用户文档。**
