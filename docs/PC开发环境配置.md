# PC 端开发环境配置（Wails + Go 桌面应用）

> 本文档记录「待办清单 TodoList」桌面版（todolist 仓库）在开发机上的完整环境配置，
> 供之后用**同一套方式**开发新 GUI 软件时直接复用。

## 一、技术栈概览

| 项 | 选型 | 说明 |
|---|---|---|
| GUI 框架 | Wails v2.15.0 | Go 后端 + 系统 WebView 渲染前端 |
| 后端语言 | Go 1.25.0 | 便携版安装，无需管理员权限 |
| SQLite 驱动 | modernc.org/sqlite v1.57.0 | **纯 Go 实现，无 CGO**，不需要装 gcc/MinGW |
| 前端 | 原生 HTML/CSS/JS | 放在 `frontend/dist/`，**无 npm、无打包器**，`wails build` 直接嵌入 exe |
| 附加依赖 | fyne.io/systray（托盘）、golang.org/x/sys（注册表/全局快捷键） | 见 go.mod |

特点：构建产物是单个绿色 exe，数据存 exe 同目录，拷贝文件夹即可迁移。

## 二、本机已装工具与路径

| 工具 | 版本 | 路径 | 说明 |
|---|---|---|---|
| Go（便携版） | 1.25.0 | `C:\Users\kanji\go-sdk\go\bin\go.exe` | 从 golang.google.cn 下载的 zip 解压，**未写入系统 PATH** |
| Wails CLI | v2.15.0 | `C:\Users\kanji\go\bin\wails.exe` | 已装好，无需重装 |
| GOPATH（模块缓存） | — | `C:\Users\kanji\go` | go mod 下载的包都在 `pkg\mod` 里 |
| Node.js | v24.19.0 | `D:\Program\nodejs` | PC 端构建**不需要**；安卓版（Capacitor）才用 |
| WebView2 运行时 | 系统自带 | — | Win10/11 自带，运行时依赖 |

## 三、构建环境变量（每次开新终端都要设）

```powershell
$env:PATH       = 'C:\Users\kanji\go-sdk\go\bin;C:\Users\kanji\go\bin;' + $env:PATH
$env:GOPROXY    = 'https://goproxy.cn,direct'   # 国内模块代理，默认的 proxy.golang.org 连不上
$env:GOTOOLCHAIN = 'local'                       # 禁止 go 自动下载别的工具链版本
```

> 嫌每次设麻烦可一劳永逸：把 `C:\Users\kanji\go-sdk\go\bin` 和 `C:\Users\kanji\go\bin`
> 加入「系统设置 → 环境变量 → 用户 PATH」，GOPROXY 加为用户变量。

检查环境是否就绪：

```powershell
go version      # 应输出 go1.25.0
wails version   # 应输出 v2.15.0
wails doctor    # 体检：依赖、WebView2 等
```

## 四、新建一个同方式的项目（推荐流程）

**方式 A（最快，实测可靠）：复制 todolist 工程做模板**

1. 整个复制 `todolist` 文件夹，重命名（删除 `release/`、`build/bin/`、`.git`）
2. 全局替换 `module todolist` → `module 新名字`（go.mod 第一行）
3. 改 `main.go` 里的窗口标题/尺寸、`wails.json` 的 `name`/`outputfilename`
4. 删掉不要的 Go 文件（托盘 tray.go、快捷键 hotkey_windows.go、自启 autostart.go
   都是可选组件，新项目不需要就直接删，同时删掉 app.go 里对应调用）
5. `wails build` 出 exe

**方式 B：标准初始化**

```powershell
wails init -n 项目名 -t vanilla   # vanilla 模板 = 原生 JS 前端
```

生成的工程里 `frontend/` 带 package.json（vite），若和本项目一样想免 npm：
把 `wails.json` 的 `"frontend:install"` 和 `"frontend:build"` 置空字符串，
前端直接手写在 `frontend/dist/`，Go 侧用 `//go:embed all:frontend/dist` 嵌入。

## 五、构建与运行

```powershell
wails build     # 产物: build/bin/<项目名>.exe（生产版）
wails dev       # 开发模式：热重载 + 浏览器调试（localhost:34115 可连 Go 后端）
```

首次 build 下载模块较慢（走 goproxy.cn），之后增量构建约 3~25 秒。

## 六、踩坑记录（重要）

1. **SQLite 驱动选型**：必须用 `modernc.org/sqlite`（纯 Go）。
   网上教程常见的 `mattn/go-sqlite3` 需要 CGO + gcc，本机没装 C 工具链会编译失败。
2. **GOPROXY 必须换国内源**：默认 proxy.golang.org 被墙，不设会卡在模块下载。
3. **Go 不在系统 PATH**：本机 Go 是便携版，新开终端必须先设 PATH（见第三节），
   否则 `wails build` 报 go 找不到。
4. **网络代理工具（S302/Watt Toolkit）共存**：它通过 hosts + 本地反代加速 github.com，
   与 Go 构建不冲突；go 模块下载走 goproxy.cn 即可，无需额外配置。
5. **前端就是源码**：本项目 `frontend/dist` 是手写文件且已提交 git
   （.gitignore 早期误配已修正）；`.gitignore` 里不要再忽略 `frontend/dist`。
6. **个人数据不入库**：`*.db`（todolist.db）和 `release/` 已在 .gitignore，
   新项目沿用，避免把用户数据传上 GitHub。

## 七、相关路径速查

| 内容 | 路径 |
|---|---|
| 本文档所属项目 | `C:\Users\kanji\Desktop\todolist` |
| 安卓版姊妹项目 | `C:\Users\kanji\Desktop\todolistapk`（Capacitor 7，见其 README.md） |
| 便携 Go | `C:\Users\kanji\go-sdk\`（不要可整文件夹删除） |
| gh CLI / 便携 git | `C:\Users\kanji\dev-tools\`（已登录 GitHub 账号 Okikan） |
| Android SDK | `C:\Users\kanji\android-sdk\`（安卓版构建用） |
