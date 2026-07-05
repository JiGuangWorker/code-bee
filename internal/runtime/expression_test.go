package runtime

import (
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

func newTestEC() *ExecutionContext {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.SetResult("review", &StepResult{
		Data: map[string]any{
			"status":      "PASS",
			"summary":     "all good",
			"tags":        []any{"bug", "critical"},
			"check_result": map[string]any{
				"passed":  true,
				"count":   42,
				"missing": "",
			},
		},
	})
	ec.SetResult("coding", &StepResult{
		Data: map[string]any{
			"status":  "DONE",
			"count":   5,
			"summary": "implemented feature X",
		},
	})
	return ec
}

func TestEvalExitCondition_Equals(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{Stage: "review", Field: "status", Operator: "equals", Value: "PASS"}
	got, err := EvalExitCondition(ec, cond)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("equals PASS should match")
	}

	cond.Value = "FAIL"
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("equals FAIL should not match PASS")
	}
}

func TestEvalExitCondition_NotEquals(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{Stage: "review", Field: "status", Operator: "not_equals", Value: "FAIL"}
	got, _ := EvalExitCondition(ec, cond)
	if !got {
		t.Error("not_equals FAIL should match when status is PASS")
	}

	cond.Value = "PASS"
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("not_equals PASS should not match when status is PASS")
	}
}

func TestEvalExitCondition_In(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{
		Stage:    "review",
		Field:    "status",
		Operator: "in",
		Value:    []any{"PASS", "BLOCKED"},
	}
	got, _ := EvalExitCondition(ec, cond)
	if !got {
		t.Error("status PASS should be in [PASS, BLOCKED]")
	}

	cond.Value = []any{"FAIL", "UNKNOWN"}
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("status PASS should not be in [FAIL, UNKNOWN]")
	}
}

func TestEvalExitCondition_Contains_String(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{
		Stage:    "coding",
		Field:    "summary",
		Operator: "contains",
		Value:    "feature",
	}
	got, _ := EvalExitCondition(ec, cond)
	if !got {
		t.Error("summary should contain 'feature'")
	}

	cond.Value = "missing-keyword"
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("summary should not contain 'missing-keyword'")
	}
}

func TestEvalExitCondition_Contains_Array(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{
		Stage:    "review",
		Field:    "tags",
		Operator: "contains",
		Value:    "critical",
	}
	got, _ := EvalExitCondition(ec, cond)
	if !got {
		t.Error("tags should contain 'critical'")
	}

	cond.Value = "enhancement"
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("tags should not contain 'enhancement'")
	}
}

func TestEvalExitCondition_Exists(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{Stage: "review", Field: "status", Operator: "exists"}
	got, _ := EvalExitCondition(ec, cond)
	if !got {
		t.Error("review.status should exist")
	}

	cond.Field = "nonexistent"
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("review.nonexistent should not exist")
	}
}

func TestEvalExitCondition_Regex(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{
		Stage:    "review",
		Field:    "status",
		Operator: "regex",
		Value:    "^P.*S$",
	}
	got, _ := EvalExitCondition(ec, cond)
	if !got {
		t.Error("status PASS should match ^P.*S$")
	}

	cond.Value = "^F.*$"
	got, _ = EvalExitCondition(ec, cond)
	if got {
		t.Error("status PASS should not match ^F.*$")
	}
}

func TestEvalExitCondition_FieldNotFound(t *testing.T) {
	ec := newTestEC()

	cond := schema.ExitCondition{Stage: "review", Field: "missing", Operator: "equals", Value: "x"}
	got, _ := EvalExitCondition(ec, cond)
	if got {
		t.Error("equals on missing field should be false")
	}

	cond.Operator = "not_equals"
	got, _ = EvalExitCondition(ec, cond)
	if !got {
		t.Error("not_equals on missing field should be true")
	}
}

func TestEvalExitCondition_UnknownOperator(t *testing.T) {
	ec := newTestEC()
	cond := schema.ExitCondition{Stage: "review", Field: "status", Operator: "bogus", Value: "x"}
	_, err := EvalExitCondition(ec, cond)
	if err == nil {
		t.Fatal("expected error for unknown operator")
	}
}

func TestEvalExpr_RoundGE(t *testing.T) {
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)
	ec.PushLoop(&LoopState{Round: 3, MaxIterations: 5})

	cases := []struct {
		expr string
		want bool
	}{
		{"round >= 2", true},
		{"round >= 3", true},
		{"round >= 4", false},
		{"round > 2", true},
		{"round > 3", false},
		{"round <= 3", true},
		{"round <= 2", false},
		{"round < 4", true},
		{"round < 3", false},
	}
	for _, c := range cases {
		got, err := EvalExpr(c.expr, ec)
		if err != nil {
			t.Errorf("EvalExpr(%q) error: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("EvalExpr(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestEvalExpr_StatusEquals(t *testing.T) {
	ec := newTestEC()

	got, err := EvalExpr(`review.status == "PASS"`, ec)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error(`review.status == "PASS" should be true`)
	}

	got, _ = EvalExpr(`review.status != "BLOCKED"`, ec)
	if !got {
		t.Error(`review.status != "BLOCKED" should be true`)
	}
}

func TestEvalExpr_NestedField(t *testing.T) {
	ec := newTestEC()

	got, err := EvalExpr(`review.check_result.count >= 40`, ec)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("review.check_result.count >= 40 should be true")
	}

	got, _ = EvalExpr(`review.check_result.count >= 50`, ec)
	if got {
		t.Error("review.check_result.count >= 50 should be false")
	}
}

func TestEvalExpr_InvalidExpression(t *testing.T) {
	ec := newTestEC()

	_, err := EvalExpr("not a valid expression", ec)
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestEvalCondition_Nil(t *testing.T) {
	ec := newTestEC()
	got, err := EvalCondition(ec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("nil condition should be true")
	}
}

func TestEvalCondition_Expr(t *testing.T) {
	ec := newTestEC()
	cond := &schema.Condition{Expr: `review.status == "PASS"`}
	got, err := EvalCondition(ec, cond)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expr condition should be true")
	}
}

func TestEvalCondition_Structured(t *testing.T) {
	ec := newTestEC()
	cond := &schema.Condition{
		Structured: &schema.ExitCondition{
			Stage: "review", Field: "status", Operator: "equals", Value: "PASS",
		},
	}
	got, err := EvalCondition(ec, cond)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("structured condition should be true")
	}
}

func TestAnyExitConditionMatched(t *testing.T) {
	ec := newTestEC()

	conds := []schema.ExitCondition{
		{Stage: "review", Field: "status", Operator: "equals", Value: "FAIL"},
		{Stage: "review", Field: "status", Operator: "equals", Value: "PASS"},
	}
	got, err := AnyExitConditionMatched(ec, conds)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("should match when one condition is satisfied")
	}

	conds = []schema.ExitCondition{
		{Stage: "review", Field: "status", Operator: "equals", Value: "FAIL"},
		{Stage: "review", Field: "status", Operator: "equals", Value: "UNKNOWN"},
	}
	got, _ = AnyExitConditionMatched(ec, conds)
	if got {
		t.Error("should not match when no condition is satisfied")
	}
}

func TestAnyExitConditionMatched_Empty(t *testing.T) {
	ec := newTestEC()
	got, err := AnyExitConditionMatched(ec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("empty conditions should not match")
	}
}
