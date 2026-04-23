package takeover

import (
	"errors"
	"strings"
	"testing"
)

// TestRunSteps_AllOK verifies that when every Do succeeds, no Undo is invoked
// and the steps execute in declared order.
func TestRunSteps_AllOK(t *testing.T) {
	var order []string
	mk := func(name string) Step {
		return Step{
			Name: name,
			Do:   func() error { order = append(order, "do:"+name); return nil },
			Undo: func() error { order = append(order, "undo:"+name); return nil },
		}
	}
	log := &Log{}
	err := RunSteps(log, []Step{mk("A"), mk("B"), mk("C")})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := strings.Join(order, ",")
	want := "do:A,do:B,do:C"
	if got != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

// TestRunSteps_MidFailureRollsBack checks that when the third step's Do fails,
// Undo is invoked for steps 2 and 1 (reverse order), but NOT for step 3.
func TestRunSteps_MidFailureRollsBack(t *testing.T) {
	var order []string
	ok := func(name string) Step {
		return Step{
			Name: name,
			Do:   func() error { order = append(order, "do:"+name); return nil },
			Undo: func() error { order = append(order, "undo:"+name); return nil },
		}
	}
	boom := Step{
		Name: "C",
		Do:   func() error { order = append(order, "do:C"); return errors.New("boom") },
		Undo: func() error { order = append(order, "undo:C"); return nil },
	}
	log := &Log{}
	err := RunSteps(log, []Step{ok("A"), ok("B"), boom})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "C:") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error %q lacks step-name prefix", err)
	}
	got := strings.Join(order, ",")
	want := "do:A,do:B,do:C,undo:B,undo:A"
	if got != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

// TestRunSteps_NilUndoSkipped verifies that steps with nil Undo don't crash
// the rollback loop and are simply skipped.
func TestRunSteps_NilUndoSkipped(t *testing.T) {
	var order []string
	log := &Log{}
	err := RunSteps(log, []Step{
		{Name: "A", Do: func() error { order = append(order, "do:A"); return nil }},
		{Name: "B", Do: func() error { order = append(order, "do:B"); return nil },
			Undo: func() error { order = append(order, "undo:B"); return nil }},
		{Name: "C", Do: func() error { return errors.New("x") }},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	got := strings.Join(order, ",")
	want := "do:A,do:B,undo:B"
	if got != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
}
