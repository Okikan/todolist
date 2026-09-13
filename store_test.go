package main

import (
	"testing"
)

// 回归测试：修复「恢复任务死锁」——事务内调用 s.db.maxPosition 导致单连接池卡死。
// 若死锁回归，此测试会超时挂起（go test -timeout 会捕获）。
func TestDeleteThenRestore(t *testing.T) {
	s, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.db.Close()

	a, err := s.AddTodo("任务A", nil, 3, "", "")
	if err != nil {
		t.Fatalf("AddTodo A: %v", err)
	}
	b, err := s.AddTodo("任务B", nil, 3, "备注B", "工作")
	if err != nil {
		t.Fatalf("AddTodo B: %v", err)
	}

	// 删除 B → 进回收站
	if err := s.DeleteTodo(b.ID); err != nil {
		t.Fatalf("DeleteTodo: %v", err)
	}
	todos, err := s.GetTodos()
	if err != nil {
		t.Fatalf("GetTodos after delete: %v", err)
	}
	if len(todos) != 1 || todos[0].ID != a.ID {
		t.Fatalf("after delete expected only A, got %d todos", len(todos))
	}

	// 恢复 B（修复前这里死锁）
	trash, err := s.GetTrash()
	if err != nil {
		t.Fatalf("GetTrash: %v", err)
	}
	if len(trash) != 1 {
		t.Fatalf("expected 1 trash item, got %d", len(trash))
	}
	if err := s.RestoreFromTrash(trash[0].ID); err != nil {
		t.Fatalf("RestoreFromTrash: %v", err)
	}

	// 验证恢复结果：字段完整（恢复使用新的自增 ID）、排在末尾
	todos, err = s.GetTodos()
	if err != nil {
		t.Fatalf("GetTodos after restore: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos after restore, got %d", len(todos))
	}
	rb := todos[1]
	if rb.Title != "任务B" || rb.Note != "备注B" || rb.Tag != "工作" || rb.Done {
		t.Fatalf("restored task fields wrong: %+v", rb)
	}

	// 删除已完成任务再恢复：完成状态保持
	if _, err := s.ToggleTodo(rb.ID); err != nil {
		t.Fatalf("ToggleTodo: %v", err)
	}
	if err := s.DeleteTodo(rb.ID); err != nil {
		t.Fatalf("DeleteTodo done: %v", err)
	}
	trash, _ = s.GetTrash()
	if err := s.RestoreFromTrash(trash[0].ID); err != nil {
		t.Fatalf("RestoreFromTrash done: %v", err)
	}
	todos, _ = s.GetTodos()
	rb2 := todos[len(todos)-1]
	if rb2.Title != "任务B" || !rb2.Done || rb2.CompletedAt == nil {
		t.Fatalf("restored done task lost its done state: %+v", rb2)
	}
}

// 回归测试：回收站 30 条上限
func TestTrashCap30(t *testing.T) {
	s, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.db.Close()

	for i := 0; i < 35; i++ {
		tk, err := s.AddTodo("临时任务", nil, 3, "", "")
		if err != nil {
			t.Fatalf("AddTodo %d: %v", i, err)
		}
		if err := s.DeleteTodo(tk.ID); err != nil {
			t.Fatalf("DeleteTodo %d: %v", i, err)
		}
	}
	trash, err := s.GetTrash()
	if err != nil {
		t.Fatalf("GetTrash: %v", err)
	}
	if len(trash) != 30 {
		t.Fatalf("trash cap: expected 30, got %d", len(trash))
	}
}
