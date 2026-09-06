# 待办清单 TodoList

一个 Windows 桌面待办事项应用，主打**截止时间管理**：任务按紧迫度自动排序、督促期高亮置顶、月历总览截止分布。

基于 [Wails v2](https://wails.io)（Go + WebView）构建，纯 Go SQLite 驱动（modernc.org/sqlite，无 CGO），绿色便携——数据库存放在 exe 同目录，整个文件夹拷走即可带走全部数据。

> 📱 安卓版见姊妹项目（Capacitor + JS 移植版，数据结构互通）。

## 功能

- **新建任务**：描述（必填）、标签、截止时间（选填）、督促时间、备注（选填）
- **紧迫度自动排序**：督促中的任务置顶（⚡ 标记 + 浅红卡片）→ 有截止时间的按时间从近到远 → 无截止时间的垫底
- **督促机制**：每条任务可单独设置"截止前 N 天开始督促"（默认 3 天），进入督促期自动标记置顶
- **视图**：未完成 / 已完成（按年月归档）/ 月历（截止分布，跟手滑动切换月份）
- **标签系统**：默认"工作 / 生活"，可自定义标签颜色，列表与月历均可按标签筛选
- **回收站**：删除的任务保留最近 30 条，可恢复或彻底删除
- **搜索与筛选**：关键词过滤 + 本周截止 / 已逾期 / 督促中快捷筛选
- **系统托盘**：关闭窗口即隐藏到托盘，托盘菜单可新建任务 / 退出；单实例锁
- **全局快捷键**：`Ctrl + Alt + Space` 呼出/隐藏窗口；应用内 `Ctrl + N` 新建任务（可自定义）
- **设置**：深色模式、快捷键自定义、开机自启，全部持久化到 SQLite
- **数据备份**：导出 / 导入 JSON（导入自动跳过完全重复的记录）
- **自动重检**：每分钟按当前时间重算督促 / 逾期状态，窗口久开也不会漏检

## 构建

需要 Go 1.25+ 和 [Wails CLI](https://wails.io/docs/gettingstarted/installation)：

```bash
wails build
```

产物在 `build/bin/todolist.exe`。前端为手写原生 HTML/CSS/JS（`frontend/dist/`），无 npm 构建步骤，改完直接 `wails build` 即可。

## 项目结构

```
├─ main.go               Wails 入口（窗口配置、单实例锁、关闭拦截）
├─ app.go                绑定给前端的方法 + 托盘/快捷键/窗口管理
├─ store.go              SQLite 持久化：CRUD、紧迫度排序、督促判定、回收站、设置、标签
├─ tray.go               系统托盘（fyne.io/systray）
├─ hotkey_windows.go     全局快捷键（Win32 RegisterHotKey）
├─ autostart.go          开机自启（HKCU Run 注册表）
└─ frontend/dist/        前端（原生 HTML/CSS/JS，无框架）
```

## 使用注意

- 数据保存在 exe 同目录的 `todolist.db`，**不要只拷 exe 删数据库**
- 点窗口 × 是隐藏到托盘，真正退出请用托盘右键菜单的"退出"
- 需要 Windows 10 / 11（系统自带 WebView2）
