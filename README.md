# Miku Chrono 🎵⏱️

> 多活动打卡计时桌面应用 —— 一只常驻桌面的「初音」计时小助手

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails3-v3.0.0--beta.16-DF4A32?logo=wails&logoColor=white)
![Vue](https://img.shields.io/badge/Vue-3-42B883?logo=vue.js&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-Pure_Go-003B57?logo=sqlite&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?logo=windows&logoColor=white)
[![CI](https://github.com/Elari39/miku-chrono/actions/workflows/ci.yml/badge.svg)](https://github.com/Elari39/miku-chrono/actions/workflows/ci.yml)

---

## 📖 命名由来

「Miku」取自 **初音未来（初音ミク / Hatsune Miku）** —— 世界的第一个虚拟歌姬，"初音"意为"最初的音符"，象征每一段时光的开始；「Chrono」源自希腊语 **χρόνος（时间）**，意为"时间的记录者"。

合在一起，Miku Chrono 寓意 **「以初音相伴，记录每一刻时光」**：一只 Q 版初音未来桌宠常驻桌面，安静地陪伴你打卡、计时、回顾每一天的投入。

## ✨ 功能特性

### ⏱️ 打卡计时

- **互斥单一计时器**：同一时间只计一个活动，切换活动时自动结算并记录上一段
- **计时记忆**：停止计时后，托盘悬停提示上次的活动与累计时长（如 `阅读 45:32`）；点击「开始计时」**延续上次的活动与累计时长**继续计时
- **分段记账，统计准确**：每次停止只把本次续计的时段写入记录，累计总长仅作展示与续计，统计与打卡数据不会被重复放大
- 误触保护：不足 1 秒的会话自动丢弃，不产生垃圾记录

### 🐬 桌宠初音（桌面常驻）

- Q 版初音未来：透明无边框、始终置顶、可自由拖拽，**位置自动记忆**（旧版悬浮球位置自动迁移）
- **六种状态 + 实时信息泡**，用动作与表情反映计时：
  - 待机呼吸 · 计时中慢跑 · 被拖拽时惊讶举手
  - 未计时超过 5 分钟会抱着膝盖打瞌睡，开始计时或点击她就醒来
  - 单击打招呼，计时开始瞬间也会挥手庆祝，目标达成瞬间举手欢呼
  - 头顶小气泡实时显示当前活动与用时（未计时显示上次活动），隐藏主窗口也能瞄一眼进度
- **随机小动作**：待机与计时中不定时触发——蹦跳、转身张望、扭一扭跳舞、伸懒腰、原地小碎步、点头打拍子、扭屁股、开心膨胀、歪头好奇
- **专注里程碑**：连续计时每满 25 分钟，Miku 伸个懒腰提醒你起来休息
- **目标达成彩蛋**：任一活动实时达到每日目标的瞬间，Miku 举手欢呼、天降彩纸，并进入两分钟的开心模式（小动作明显变勤）
- **镜像跑步动画**：计时中的奔跑由同一立绘左右镜像快速交替——摆臂和迈腿必然换边、形象零漂移——配合一步一颠的小弹跳，是真的在跑
- 左键**双击**：显示/隐藏主窗口；鼠标**悬停**在她身上会有小小的雀跃反应
- 计时状态与主窗口**秒级同步**（任意一端开始/停止，另一端即刻刷新）
- 素材可替换：支持 AI 生成的透明底立绘，见 [docs/pet-assets.md](docs/pet-assets.md)

### 🗂️ 活动与分类

- 活动卡片：名称、颜色、图标、每日目标分钟数
- 编辑 / 归档 / 删除，支持自定义分类归纳

### 📊 记录与统计

- 记录列表：分页、按活动/日期筛选，支持手动补记、编辑与删除
- 统计页：**日 / 周 / 月 / 年**四个维度自由切换，每日（每月）专注堆积柱状图
- 日视图附 **24 小时时间轴**：跨午夜记录按当天切片展示，并可一键跳转筛选后的记录列表
- **连续打卡（streak）**：当前与最长连续天数，附日均时长、峰值日等汇总卡片

### 🧩 系统托盘

- 常驻通知区，保证应用永远可寻回
- 动态菜单：计时中 `停止计时 · MM:SS`，空闲 `开始计时 · 上次时长`
- 左键单击恢复主窗口

### 🔔 提醒与系统集成

- **每日目标提醒**：为活动设定每日目标分钟数，运行中的计时实时计入进度（打卡页进度条每秒走动）；达成时弹出系统气泡通知，桌宠同时撒花庆祝（每活动每天至多一次，设置中可关闭）
- **开机自启动**：注册 Windows 自启动项，并附带静默启动标记（开机仅唤起托盘与桌宠）
- **窗口位置记忆**：主窗口位置/尺寸与桌宠位置跨启动恢复，并自动钳制到可见屏幕——拔掉外接显示器后窗口也不会跑到屏幕外
- **关闭行为**：点主窗口关闭键默认隐藏到后台，可在设置改为直接退出
- 单实例运行：重复启动会唤醒已运行的实例，不会开出第二个进程

### 💾 数据与设置

- 数据存于本地 SQLite（纯 Go 驱动，无 CGO 依赖）
- JSON / CSV（Excel 友好）一键导出备份
- 设置页：桌宠显示/隐藏、关闭行为、开机自启动、每日目标提醒开关
- 危险操作保护：清空全部记录需两步确认，删除活动前明确提示记录将一并删除

## 🛠️ 技术栈

| 层 | 技术 |
| --- | --- |
| 桌面框架 | [Wails v3](https://v3.wails.io/)（v3.0.0-beta.16） |
| 后端语言 | Go 1.25 |
| 数据库 | SQLite（[modernc.org/sqlite](https://modernc.org/sqlite)，纯 Go 实现） |
| 前端框架 | Vue 3 + TypeScript + Vite + Tailwind CSS 4 + vue-router |
| 绑定层 | Wails 自动生成 TS 绑定（`frontend/bindings`） |

## 🚀 快速开始

### 平台支持

本项目**当前仅支持 Windows**：构建、测试与发布均以 Windows 为准。

### 环境要求

- Windows 10/11
- Go 1.25+
- Node.js（含 pnpm）
- [wails3 CLI](https://v3.wails.io/docs/next/gettingstarted/installation/)（版本需与 `go.mod` 匹配）

### 开发模式（热重载）

```bash
wails3 dev
```

### 构建

```bash
wails3 build
# 产物：bin/miku-chrono.exe（Windows）

wails3 task package
# 额外生成 NSIS 安装包：bin/miku-chrono-amd64-installer.exe
```

> 构建流水线会自动生成前端产物（`frontend/dist`）与 TS 绑定，无需手动干预。

### 版本号约定

应用版本维护在两处，更新时需同步：

- `build/config.yml` 的 `info.version` —— 可执行文件与 NSIS 安装包的元数据
- `frontend/package.json` 的 `version` —— 设置页展示的应用版本

更新 `build/config.yml` 后运行 `wails3 task common:update:build-assets` 同步 `build/` 下的安装包与资源清单，再一并提交。

### 测试

```bash
wails3 task test      # go vet + go test，随后前端 lint + vitest
wails3 task lint      # golangci-lint（需自行安装）
```

也可以分开手动运行：

```bash
go vet ./...
go test ./...         # store / services / applog 共 60+ 个测试
cd frontend
pnpm lint             # ESLint
pnpm test             # vitest：lib 纯函数、组件与 composables（90+ 个用例）
```

推送与 PR 由 GitHub Actions（`.github/workflows/ci.yml`，Windows runner）自动执行以上检查。

## 📁 项目结构

```text
Miku_Chrono/
├── main.go                 # 入口：主窗口 / 桌宠 / 右键菜单 / 单实例 / 关闭行为装配
├── tray.go                 # 系统托盘 + 菜单状态同步泵（1s 轮询，内存快照投影）
├── shell_windows.go        # 平台钩子：Explorer 打开目录 / 原生保存对话框
├── DESIGN.md               # 设计系统规范（暖奶油画布 + 珊瑚强调色）
├── docs/pet-assets.md      # 桌宠素材规格、再生成脚本用法与版权说明
├── tools/gen-pet-assets/   # 桌宠立绘 AI 生成脚本（密钥只走环境变量）
├── internal/
│   ├── models/             # Go ↔ TypeScript 共享数据结构
│   ├── store/              # SQLite 数据层：迁移 / 种子数据 / 全部 SQL
│   ├── applog/             # 最小文件日志（后台 goroutine 错误留痕，1MB 轮转）
│   └── services/           # Wails 绑定服务：校验 / 统计 / 导出 / 桌宠 / 提醒
│                           #   平台钩子也在本层：notify / autostart / 窗口钳制（_windows.go）
└── frontend/
    ├── src/                # Vue 3 应用
    │   ├── views/          # 打卡 / 统计 / 记录 / 设置 / 桌宠
    │   ├── components/     # 通用组件（计时卡、图表、弹窗、表单等）
    │   ├── composables/    # useTimer / useToday / useVersionedLoad / useToast 等
    │   ├── lib/            # API 汇总与纯函数工具（格式化 / 统计周期 / 调色板 / 素材加载）
    │   └── assets/pet/     # 桌宠立绘（AI 生成）与素材许可说明
    └── bindings/           # 由 wails3 自动生成的绑定（请勿手改）
```

## 🎨 设计语言

暖奶油色画布（`#faf9f5`）× 珊瑚强调色（`#cc785c`）× 深色信息面板的三元色板，配衬线展示字体与克制的阴影 —— 完整 token 与组件规范见 [DESIGN.md](DESIGN.md)。

## 🗄️ 数据存储位置

数据库文件位于用户配置目录：

| 系统 | 路径 |
| --- | --- |
| Windows | `%APPDATA%\Miku_Chrono\mikuchrono.db` |

备份优先使用应用内导出（设置 → 数据导出）。数据库处于 WAL 模式，若直接复制 `mikuchrono.db`，需同时带上 `mikuchrono.db-wal` 与 `mikuchrono.db-shm`（且最好在应用退出后复制），否则最新写入可能还在 `-wal` 里没有落盘。

## 📄 许可

本项目基于 [MIT License](LICENSE) 协议开源发布。

第三方资源许可另行说明：

- Inter 字体采用 SIL OFL 1.1，详见 `frontend/Inter Font License.txt`
- 桌宠初音未来形象基于 [Piapro Character License](https://piapro.jp/license/pcl) 使用，Hatsune Miku © Crypton Future Media, INC.，详见 `frontend/src/assets/pet/LICENSE.txt`（不属于 MIT 许可范围）
