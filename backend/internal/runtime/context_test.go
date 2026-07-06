package runtime

import (
	"testing"
)

func TestExecutionContext_Lookup_BasicField(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.SetResult("review", &StepResult{
		Data: map[string]any{
			"status": "PASS",
		},
	})

	val, ok := ec.Lookup("review", "status")
	if !ok {
		t.Fatal("expected to find review.status")
	}
	if val != "PASS" {
		t.Errorf("got %v, want PASS", val)
	}
}

func TestExecutionContext_Lookup_NestedPath(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.SetResult("review", &StepResult{
		Data: map[string]any{
			"check_result": map[string]any{
				"passed": true,
				"count":  42,
			},
		},
	})

	val, ok := ec.Lookup("review", "check_result.passed")
	if !ok {
		t.Fatal("expected to find review.check_result.passed")
	}
	if val != true {
		t.Errorf("got %v, want true", val)
	}

	val, ok = ec.Lookup("review", "check_result.count")
	if !ok {
		t.Fatal("expected to find review.check_result.count")
	}
	if val != 42 {
		t.Errorf("got %v, want 42", val)
	}
}

func TestExecutionContext_Lookup_EmptyPath(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	data := map[string]any{"status": "DONE", "summary": "ok"}
	ec.SetResult("coding", &StepResult{Data: data})

	val, ok := ec.Lookup("coding", "")
	if !ok {
		t.Fatal("expected to find coding data")
	}
	m, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if m["status"] != "DONE" {
		t.Errorf("got status %v, want DONE", m["status"])
	}
}

func TestExecutionContext_Lookup_StageNotFound(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	_, ok := ec.Lookup("nonexistent", "status")
	if ok {
		t.Fatal("expected not found for nonexistent stage")
	}
}

func TestExecutionContext_Lookup_FieldNotFound(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.SetResult("review", &StepResult{
		Data: map[string]any{"status": "PASS"},
	})

	_, ok := ec.Lookup("review", "missing")
	if ok {
		t.Fatal("expected not found for missing field")
	}
}

func TestExecutionContext_SetResult_GetResult(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	r := &StepResult{StageName: "coding", Status: "DONE"}
	ec.SetResult("coding", r)

	got, ok := ec.GetResult("coding")
	if !ok {
		t.Fatal("expected to find coding result")
	}
	if got.Status != "DONE" {
		t.Errorf("got status %s, want DONE", got.Status)
	}
}

func TestExecutionContext_Snapshot_Merge(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.SetResult("a", &StepResult{Status: "A"})
	ec.SetResult("b", &StepResult{Status: "B"})

	snap := ec.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}

	// 修改快照不影响主上下文
	snap["c"] = &StepResult{Status: "C"}
	if _, ok := ec.GetResult("c"); ok {
		t.Fatal("snapshot should not affect main context")
	}

	// Merge 回写
	ec.Merge(snap)
	got, ok := ec.GetResult("c")
	if !ok {
		t.Fatal("expected c after merge")
	}
	if got.Status != "C" {
		t.Errorf("got status %s, want C", got.Status)
	}
}

func TestExecutionContext_CloneForBranch_WriteIsolation(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.SetResult("base", &StepResult{Status: "BASE"})

	base := ec.Snapshot()
	branch := ec.CloneForBranch(base)

	// 分支写入不影响主上下文
	branch.SetResult("branch-only", &StepResult{Status: "BRANCH"})
	if _, ok := ec.GetResult("branch-only"); ok {
		t.Fatal("branch write should not affect main context")
	}

	// 主上下文写入不影响分支
	ec.SetResult("main-only", &StepResult{Status: "MAIN"})
	if _, ok := branch.GetResult("main-only"); ok {
		t.Fatal("main write should not affect branch context")
	}

	// 基线数据两边都可见
	got, ok := branch.GetResult("base")
	if !ok {
		t.Fatal("branch should see base result")
	}
	if got.Status != "BASE" {
		t.Errorf("got status %s, want BASE", got.Status)
	}
}

func TestExecutionContext_PushLoop_PopLoop(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)

	if ec.CurrentLoop() != nil {
		t.Fatal("expected nil loop on empty stack")
	}

	outer := &LoopState{ID: "outer", Round: 1, MaxIterations: 3}
	ec.PushLoop(outer)

	got := ec.CurrentLoop()
	if got == nil || got.ID != "outer" {
		t.Fatalf("expected outer loop, got %v", got)
	}

	inner := &LoopState{ID: "inner", Round: 1, MaxIterations: 2}
	ec.PushLoop(inner)

	got = ec.CurrentLoop()
	if got == nil || got.ID != "inner" {
		t.Fatalf("expected inner loop, got %v", got)
	}

	ec.PopLoop()
	got = ec.CurrentLoop()
	if got == nil || got.ID != "outer" {
		t.Fatalf("expected outer loop after pop, got %v", got)
	}

	ec.PopLoop()
	if ec.CurrentLoop() != nil {
		t.Fatal("expected nil loop after popping all")
	}
}

func TestSplitDotPath(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"status", []string{"status"}},
		{"a.b.c", []string{"a", "b", "c"}},
		{"", []string{}},
		{"a", []string{"a"}},
	}
	for _, c := range cases {
		got := splitDotPath(c.path)
		if len(got) != len(c.want) {
			t.Errorf("splitDotPath(%q) = %v, want %v", c.path, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitDotPath(%q)[%d] = %q, want %q", c.path, i, got[i], c.want[i])
			}
		}
	}
}

func TestLoopState_String(t *testing.T) {
	s := &LoopState{
		ID:                    "dev-loop",
		Round:                 2,
		MaxIterations:         3,
		ConsecutiveUnknown:    1,
		MaxConsecutiveUnknown: 2,
	}
	str := s.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
}
