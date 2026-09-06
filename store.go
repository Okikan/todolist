package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Todo 一条待办
type Todo struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Done        bool   `json:"done"`
	Position    int64  `json:"position"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt *int64 `json:"completed_at"`
	DueAt       *int64 `json:"due_at"`
	UrgeDays    int64  `json:"urge_days"` // 截止前多少天开始督促
	Note        string `json:"note"`      // 备注（选填）
	Tag         string `json:"tag"`       // 标签名（选填）
	Urge        bool   `json:"urge"`      // 派生字段：是否处于督促期（不落库）
}

// Settings 应用设置
type Settings struct {
	DarkMode      bool   `json:"dark_mode"`
	NewTaskHotkey string `json:"new_task_hotkey"`
}

// TagDef 标签定义（名称 + 颜色）
type TagDef struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TrashItem 回收站中的一条记录
type TrashItem struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Done        bool   `json:"done"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt *int64 `json:"completed_at"`
	DueAt       *int64 `json:"due_at"`
	UrgeDays    int64  `json:"urge_days"`
	Note        string `json:"note"`
	Tag         string `json:"tag"`
	DeletedAt   int64  `json:"deleted_at"`
}

// Store 负责 SQLite 持久化
type Store struct {
	db *sql.DB
}

// NewStore 打开（或创建）exe 同目录下的 todolist.db，保证整包可移植
func NewStore() (*Store, error) {
	dir := executableDir()
	dbPath := filepath.Join(dir, "todolist.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，避免锁冲突
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		done INTEGER NOT NULL DEFAULT 0,
		position INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		completed_at INTEGER,
		due_at INTEGER,
		urge_days INTEGER NOT NULL DEFAULT 3,
		note TEXT,
		tag TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		db.Close()
		return nil, err
	}
	// 旧版本数据库升级：补列（列已存在时忽略报错）
	for _, ddl := range []string{
		`ALTER TABLE todos ADD COLUMN due_at INTEGER`,
		`ALTER TABLE todos ADD COLUMN urge_days INTEGER NOT NULL DEFAULT 3`,
		`ALTER TABLE todos ADD COLUMN note TEXT`,
		`ALTER TABLE todos ADD COLUMN tag TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(ddl); err != nil &&
			!strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	// 回收站：保留最近 30 条删除记录
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS trash (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		done INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		completed_at INTEGER,
		due_at INTEGER,
		urge_days INTEGER NOT NULL DEFAULT 3,
		note TEXT,
		tag TEXT NOT NULL DEFAULT '',
		deleted_at INTEGER NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}

// AddTodo 追加任务到末尾；dueAt 为截止时间（Unix 秒），nil 表示不设置；
// urgeDays 为截止前多少天开始督促（默认 3）；note/tag 选填
func (s *Store) AddTodo(title string, dueAt *int64, urgeDays int64, note string, tag string) (Todo, error) {
	if urgeDays < 0 {
		urgeDays = 0
	}
	pos, err := s.maxPosition()
	if err != nil {
		return Todo{}, err
	}
	res, err := s.db.Exec(
		`INSERT INTO todos (title, done, position, created_at, due_at, urge_days, note, tag) VALUES (?, 0, ?, ?, ?, ?, ?, ?)`,
		title, pos+1, time.Now().Unix(), dueAt, urgeDays, note, tag,
	)
	if err != nil {
		return Todo{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Todo{}, err
	}
	return s.GetTodo(id)
}

// UpdateTodo 编辑任务：更新标题、截止时间、督促时间、备注与标签
func (s *Store) UpdateTodo(id int64, title string, dueAt *int64, urgeDays int64, note string, tag string) (Todo, error) {
	if urgeDays < 0 {
		urgeDays = 0
	}
	res, err := s.db.Exec(
		`UPDATE todos SET title = ?, due_at = ?, urge_days = ?, note = ?, tag = ? WHERE id = ?`,
		title, dueAt, urgeDays, note, tag, id,
	)
	if err != nil {
		return Todo{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Todo{}, sql.ErrNoRows
	}
	return s.GetTodo(id)
}

// ImportTodo 插入一条导入的任务（保留原始完成状态与时间字段）
func (s *Store) ImportTodo(t Todo) error {
	_, err := s.db.Exec(
		`INSERT INTO todos (title, done, position, created_at, completed_at, due_at, urge_days)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Title, doneInt(t.Done), t.Position, t.CreatedAt, t.CompletedAt, t.DueAt, t.UrgeDays,
	)
	return err
}

// HasIdenticalTodo 判断是否已存在完全相同的任务（导入去重用）
func (s *Store) HasIdenticalTodo(t Todo) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM todos
		 WHERE title = ? AND done = ? AND position = ? AND created_at = ?
		   AND completed_at IS ? AND due_at IS ? AND urge_days = ?
		   AND IFNULL(note, '') = ? AND tag = ?`,
		t.Title, doneInt(t.Done), t.Position, t.CreatedAt, t.CompletedAt, t.DueAt, t.UrgeDays, t.Note, t.Tag,
	).Scan(&count)
	return count > 0, err
}

// ---- 标签定义 ----

const tagSettingsKey = "tags"

// 首次使用时的默认标签
var defaultTagDefs = []TagDef{
	{Name: "工作", Color: "#4a90e2"},
	{Name: "生活", Color: "#3aa76d"},
}

// GetTagDefs 读取标签定义；从未设置过时返回默认标签
func (s *Store) GetTagDefs() ([]TagDef, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, tagSettingsKey).Scan(&value)
	if err == sql.ErrNoRows {
		return append([]TagDef(nil), defaultTagDefs...), nil
	}
	if err != nil {
		return nil, err
	}
	var defs []TagDef
	if err := json.Unmarshal([]byte(value), &defs); err != nil {
		return append([]TagDef(nil), defaultTagDefs...), nil
	}
	if defs == nil {
		defs = []TagDef{}
	}
	return defs, nil
}

// SaveTagDefs 全量保存标签定义
func (s *Store) SaveTagDefs(defs []TagDef) error {
	data, err := json.Marshal(defs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, tagSettingsKey, string(data))
	return err
}

// RemoveTagEverywhere 从所有任务上移除指定标签
func (s *Store) RemoveTagEverywhere(name string) error {
	_, err := s.db.Exec(`UPDATE todos SET tag = '' WHERE tag = ?`, name)
	return err
}

func doneInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetSettings 读取应用设置（缺省：浅色主题 + Ctrl+N 新建任务）
func (s *Store) GetSettings() (Settings, error) {
	out := Settings{DarkMode: false, NewTaskHotkey: "Ctrl+N"}
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return out, err
		}
		switch k {
		case "dark_mode":
			out.DarkMode = v == "1"
		case "new_task_hotkey":
			if v != "" {
				out.NewTaskHotkey = v
			}
		}
	}
	return out, rows.Err()
}

// SaveSettings 保存应用设置
func (s *Store) SaveSettings(st Settings) error {
	dark := "0"
	if st.DarkMode {
		dark = "1"
	}
	if st.NewTaskHotkey == "" {
		st.NewTaskHotkey = "Ctrl+N"
	}
	for _, kv := range [][2]string{
		{"dark_mode", dark},
		{"new_task_hotkey", st.NewTaskHotkey},
	} {
		if _, err := s.db.Exec(
			`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, kv[0], kv[1],
		); err != nil {
			return err
		}
	}
	return nil
}

// GetTodos 未完成在上、已完成在下；未完成按督促状态与截止时间紧迫度排序
func (s *Store) GetTodos() ([]Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, title, done, position, created_at, completed_at, due_at, urge_days, note, tag
		 FROM todos ORDER BY done ASC, position ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	todos := []Todo{}
	for rows.Next() {
		var t Todo
		var done int
		var completedAt, dueAt sql.NullInt64
		var note sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &done, &t.Position, &t.CreatedAt, &completedAt, &dueAt, &t.UrgeDays, &note, &t.Tag); err != nil {
			return nil, err
		}
		t.Done = done == 1
		t.Note = note.String
		if completedAt.Valid {
			v := completedAt.Int64
			t.CompletedAt = &v
		}
		if dueAt.Valid {
			v := dueAt.Int64
			t.DueAt = &v
		}
		t.Urge = isUrging(t, time.Now().Unix())
		todos = append(todos, t)
	}
	sort.SliceStable(todos, func(i, j int) bool {
		return todoLess(todos[i], todos[j], time.Now().Unix())
	})
	return todos, rows.Err()
}

// 督促等级：0=督促中（截止时间已进入督促窗口），1=有截止时间但不紧迫，2=无截止时间
const (
	urgeRankUrging = 0
	urgeRankDue    = 1
	urgeRankNoDue  = 2
)

func isUrging(t Todo, now int64) bool {
	return t.DueAt != nil && *t.DueAt-now <= t.UrgeDays*86400
}

func urgencyRank(t Todo, now int64) int {
	switch {
	case t.DueAt == nil:
		return urgeRankNoDue
	case isUrging(t, now):
		return urgeRankUrging
	default:
		return urgeRankDue
	}
}

func (t Todo) due() int64 {
	if t.DueAt == nil {
		return 0
	}
	return *t.DueAt
}

// todoLess 未完成任务排序：督促中的置顶（按截止时间升序），其余有截止时间的按截止时间升序，
// 无截止时间的按手动顺序垫底；已完成的保持原序（由前端按年月分组）
func todoLess(a, b Todo, now int64) bool {
	if a.Done != b.Done {
		return !a.Done
	}
	if a.Done {
		return false
	}
	ra, rb := urgencyRank(a, now), urgencyRank(b, now)
	if ra != rb {
		return ra < rb
	}
	if ra == urgeRankNoDue {
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	}
	if a.due() != b.due() {
		return a.due() < b.due()
	}
	if a.Position != b.Position {
		return a.Position < b.Position
	}
	return a.ID < b.ID
}

// GetTodo 查询单条
func (s *Store) GetTodo(id int64) (Todo, error) {
	var t Todo
	var done int
	var completedAt, dueAt sql.NullInt64
	var note sql.NullString
	err := s.db.QueryRow(
		`SELECT id, title, done, position, created_at, completed_at, due_at, urge_days, note, tag FROM todos WHERE id = ?`, id,
	).Scan(&t.ID, &t.Title, &done, &t.Position, &t.CreatedAt, &completedAt, &dueAt, &t.UrgeDays, &note, &t.Tag)
	if err != nil {
		return Todo{}, err
	}
	t.Done = done == 1
	t.Note = note.String
	if completedAt.Valid {
		v := completedAt.Int64
		t.CompletedAt = &v
	}
	if dueAt.Valid {
		v := dueAt.Int64
		t.DueAt = &v
	}
	t.Urge = isUrging(t, time.Now().Unix())
	return t, nil
}

// ToggleTodo 切换完成状态；变为完成时排到已完成末尾
func (s *Store) ToggleTodo(id int64) (Todo, error) {
	t, err := s.GetTodo(id)
	if err != nil {
		return Todo{}, err
	}
	if !t.Done {
		pos, err := s.maxPosition()
		if err != nil {
			return Todo{}, err
		}
		if _, err := s.db.Exec(
			`UPDATE todos SET done = 1, position = ?, completed_at = ? WHERE id = ?`,
			pos+1, time.Now().Unix(), id,
		); err != nil {
			return Todo{}, err
		}
	} else {
		// 恢复为未完成：追加到未完成末尾
		pos, err := s.maxPosition()
		if err != nil {
			return Todo{}, err
		}
		if _, err := s.db.Exec(
			`UPDATE todos SET done = 0, position = ?, completed_at = NULL WHERE id = ?`,
			pos+1, id,
		); err != nil {
			return Todo{}, err
		}
	}
	return s.GetTodo(id)
}

// DeleteTodo 删除任务：移入回收站（保留最近 30 条）
func (s *Store) DeleteTodo(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO trash (title, done, created_at, completed_at, due_at, urge_days, note, tag, deleted_at)
		 SELECT title, done, created_at, completed_at, due_at, urge_days, note, tag, ? FROM todos WHERE id = ?`,
		time.Now().Unix(), id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM todos WHERE id = ?`, id); err != nil {
		return err
	}
	// 只保留最近 30 条
	if _, err := tx.Exec(
		`DELETE FROM trash WHERE id NOT IN (SELECT id FROM trash ORDER BY deleted_at DESC, id DESC LIMIT 30)`,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// GetTrash 回收站列表（按删除时间倒序）
func (s *Store) GetTrash() ([]TrashItem, error) {
	rows, err := s.db.Query(
		`SELECT id, title, done, created_at, completed_at, due_at, urge_days, note, tag, deleted_at
		 FROM trash ORDER BY deleted_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TrashItem{}
	for rows.Next() {
		var it TrashItem
		var done int
		var completedAt, dueAt sql.NullInt64
		var note, tag sql.NullString
		if err := rows.Scan(&it.ID, &it.Title, &done, &it.CreatedAt, &completedAt, &dueAt, &it.UrgeDays, &note, &tag, &it.DeletedAt); err != nil {
			return nil, err
		}
		it.Done = done == 1
		if completedAt.Valid {
			v := completedAt.Int64
			it.CompletedAt = &v
		}
		if dueAt.Valid {
			v := dueAt.Int64
			it.DueAt = &v
		}
		it.Note = note.String
		it.Tag = tag.String
		items = append(items, it)
	}
	return items, rows.Err()
}

// RestoreFromTrash 从回收站恢复任务（追加到对应分组末尾）
func (s *Store) RestoreFromTrash(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var it TrashItem
	var done int
	var completedAt, dueAt sql.NullInt64
	var note, tag sql.NullString
	err = tx.QueryRow(
		`SELECT title, done, created_at, completed_at, due_at, urge_days, note, tag FROM trash WHERE id = ?`, id,
	).Scan(&it.Title, &done, &it.CreatedAt, &completedAt, &dueAt, &it.UrgeDays, &note, &tag)
	if err != nil {
		return err
	}
	pos, err := s.maxPosition()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO todos (title, done, position, created_at, completed_at, due_at, urge_days, note, tag)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.Title, done, pos+1, it.CreatedAt, completedAt, dueAt, it.UrgeDays, note, tag,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM trash WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// PurgeTrash 彻底删除回收站中的一条
func (s *Store) PurgeTrash(id int64) error {
	_, err := s.db.Exec(`DELETE FROM trash WHERE id = ?`, id)
	return err
}

// ClearTrash 清空回收站
func (s *Store) ClearTrash() error {
	_, err := s.db.Exec(`DELETE FROM trash`)
	return err
}

// ReorderTodos 将 ids 列表按顺序重排（前端已按分组传入），position 从 0 开始
func (s *Store) ReorderTodos(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE todos SET position = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, id := range ids {
		if _, err := stmt.Exec(int64(i), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) maxPosition() (int64, error) {
	var pos sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(position) FROM todos`).Scan(&pos); err != nil {
		return 0, err
	}
	if !pos.Valid {
		return 0, nil
	}
	return pos.Int64, nil
}



