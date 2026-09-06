package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是暴露给前端的主结构体
type App struct {
	ctx      context.Context
	store    *Store
	hidden   bool // 主窗口当前是否已隐藏到托盘
	quitting bool // 通过托盘"退出"时置位，放行窗口真正关闭
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{}
}

// startup 在应用启动时调用：初始化数据库、托盘图标与全局快捷键
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	s, err := NewStore()
	if err != nil {
		return
	}
	a.store = s
	startTray(a)
	registerGlobalHotkey(a)
}

// onBeforeClose 点窗口关闭按钮时隐藏到托盘而不是退出；托盘"退出"才真正退出
func (a *App) onBeforeClose(ctx context.Context) bool {
	if a.quitting {
		return false
	}
	a.hideWindow()
	return true
}

// onSecondInstanceLaunch 重复启动 exe 时，直接显示已运行的窗口
func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	a.showWindow()
}

// showWindow 显示主窗口
func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	a.hidden = false
	runtime.WindowShow(a.ctx)
}

// hideWindow 隐藏主窗口
func (a *App) hideWindow() {
	if a.ctx == nil {
		return
	}
	a.hidden = true
	runtime.WindowHide(a.ctx)
}

// toggleWindow 显示/隐藏主窗口（托盘菜单与全局快捷键共用）
func (a *App) toggleWindow() {
	if a.ctx == nil {
		return
	}
	if a.hidden {
		a.showWindow()
		return
	}
	if runtime.WindowIsMinimised(a.ctx) {
		runtime.WindowUnminimise(a.ctx)
		return
	}
	a.hideWindow()
}

// ---- 绑定给前端的方法 ----

// AddTodo 新增一条待办（dueAt 为截止时间 Unix 秒，nil 表示不设置；
// urgeDays 为截止前多少天开始督促，默认 3；note/tag 选填）
func (a *App) AddTodo(title string, dueAt *int64, urgeDays int64, note string, tag string) (Todo, error) {
	return a.store.AddTodo(title, dueAt, urgeDays, note, tag)
}

// UpdateTodo 编辑任务：更新标题、截止时间、督促时间、备注与标签
func (a *App) UpdateTodo(id int64, title string, dueAt *int64, urgeDays int64, note string, tag string) (Todo, error) {
	return a.store.UpdateTodo(id, title, dueAt, urgeDays, note, tag)
}

// GetTodos 返回全部待办（后端已按紧迫度排序）
func (a *App) GetTodos() ([]Todo, error) {
	return a.store.GetTodos()
}

// ToggleTodo 在完成/未完成之间切换
func (a *App) ToggleTodo(id int64) (Todo, error) {
	return a.store.ToggleTodo(id)
}

// DeleteTodo 删除一条待办
func (a *App) DeleteTodo(id int64) error {
	return a.store.DeleteTodo(id)
}

// ReorderTodos 按 ids 顺序（同一分组内）重新排序
func (a *App) ReorderTodos(ids []int64) error {
	return a.store.ReorderTodos(ids)
}

// GetSettings 读取应用设置
func (a *App) GetSettings() (Settings, error) {
	return a.store.GetSettings()
}

// SaveSettings 保存应用设置（深色模式 + 新建任务快捷键）
func (a *App) SaveSettings(darkMode bool, newTaskHotkey string) error {
	return a.store.SaveSettings(Settings{DarkMode: darkMode, NewTaskHotkey: newTaskHotkey})
}

// ---- 标签 ----

// GetTags 获取标签定义
func (a *App) GetTags() ([]TagDef, error) {
	return a.store.GetTagDefs()
}

// SaveTags 全量保存标签定义
func (a *App) SaveTags(tags []TagDef) error {
	return a.store.SaveTagDefs(tags)
}

// DeleteTag 删除标签：同时从所有任务上移除该标签
func (a *App) DeleteTag(name string) error {
	defs, err := a.store.GetTagDefs()
	if err != nil {
		return err
	}
	out := make([]TagDef, 0, len(defs))
	for _, d := range defs {
		if d.Name != name {
			out = append(out, d)
		}
	}
	if err := a.store.SaveTagDefs(out); err != nil {
		return err
	}
	return a.store.RemoveTagEverywhere(name)
}

// ---- 回收站 ----

// GetTrash 回收站列表（最近删除的 30 条）
func (a *App) GetTrash() ([]TrashItem, error) {
	return a.store.GetTrash()
}

// RestoreTodo 从回收站恢复任务
func (a *App) RestoreTodo(id int64) error {
	return a.store.RestoreFromTrash(id)
}

// PurgeTrash 彻底删除回收站中的一条
func (a *App) PurgeTrash(id int64) error {
	return a.store.PurgeTrash(id)
}

// ClearTrash 清空回收站
func (a *App) ClearTrash() error {
	return a.store.ClearTrash()
}

// ---- 数据导入导出 ----

// ExportTodos 导出全部任务到用户选择的 JSON 文件；返回导出条数，用户取消返回 0
func (a *App) ExportTodos() (int, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出任务",
		DefaultFilename: "todolist-" + time.Now().Format("2006-01-02") + ".json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return 0, err
	}
	if path == "" {
		return 0, nil // 用户取消
	}
	todos, err := a.store.GetTodos()
	if err != nil {
		return 0, err
	}
	payload := struct {
		App        string `json:"app"`
		Version    int    `json:"version"`
		ExportedAt int64  `json:"exported_at"`
		Todos      []Todo `json:"todos"`
	}{App: "todolist", Version: 1, ExportedAt: time.Now().Unix(), Todos: todos}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, err
	}
	return len(todos), nil
}

// ImportResult 导入结果统计
type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ImportTodos 从用户选择的 JSON 文件导入任务，自动跳过完全重复的记录；
// 用户取消时返回零值
func (a *App) ImportTodos() (ImportResult, error) {
	var result ImportResult
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入任务",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return result, err
	}
	if path == "" {
		return result, nil // 用户取消
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	var payload struct {
		Todos []Todo `json:"todos"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return result, errors.New("文件格式不正确，需要本程序导出的 JSON 文件")
	}
	pos, err := a.store.maxPosition()
	if err != nil {
		return result, err
	}
	for _, t := range payload.Todos {
		if strings.TrimSpace(t.Title) == "" {
			continue
		}
		if t.UrgeDays < 0 {
			t.UrgeDays = 0
		}
		dup, err := a.store.HasIdenticalTodo(t)
		if err != nil {
			return result, err
		}
		if dup {
			result.Skipped++
			continue
		}
		pos++
		t.Position = pos
		if err := a.store.ImportTodo(t); err != nil {
			return result, err
		}
		result.Imported++
	}
	return result, nil
}
