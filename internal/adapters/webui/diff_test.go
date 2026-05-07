package webui

import "testing"

func TestDiffLines_Equal(t *testing.T) {
	t.Parallel()
	rows := DiffLines("a\nb\nc\n", "a\nb\nc\n")
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	for _, r := range rows {
		if r.Op != DiffEqual {
			t.Errorf("op=%v, want Equal", r.Op)
		}
		if r.Left != r.Right {
			t.Errorf("Left=%q Right=%q must match for Equal", r.Left, r.Right)
		}
	}
}

func TestDiffLines_Added(t *testing.T) {
	t.Parallel()
	rows := DiffLines("a\nc\n", "a\nb\nc\n")
	// expect: equal a, added b, equal c
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[1].Op != DiffAdded || rows[1].Right != "b" || rows[1].Left != "" {
		t.Errorf("rows[1]=%+v, want Added b", rows[1])
	}
}

func TestDiffLines_Removed(t *testing.T) {
	t.Parallel()
	rows := DiffLines("a\nb\nc\n", "a\nc\n")
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[1].Op != DiffRemoved || rows[1].Left != "b" || rows[1].Right != "" {
		t.Errorf("rows[1]=%+v, want Removed b", rows[1])
	}
}

func TestDiffLines_Replaced(t *testing.T) {
	t.Parallel()
	rows := DiffLines("a\nold\nc\n", "a\nnew\nc\n")
	// expect 1 removed + 1 added (or vice versa) between two equals
	var removed, added int
	for _, r := range rows {
		switch r.Op {
		case DiffRemoved:
			removed++
		case DiffAdded:
			added++
		}
	}
	if removed != 1 || added != 1 {
		t.Errorf("removed=%d added=%d, want 1/1; rows=%+v", removed, added, rows)
	}
}

func TestDiffLines_BothEmpty(t *testing.T) {
	t.Parallel()
	rows := DiffLines("", "")
	if len(rows) != 0 {
		t.Errorf("rows=%d, want 0", len(rows))
	}
}

func TestDiffLines_LeftEmpty(t *testing.T) {
	t.Parallel()
	rows := DiffLines("", "x\ny\n")
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	for _, r := range rows {
		if r.Op != DiffAdded {
			t.Errorf("op=%v, want Added", r.Op)
		}
	}
}

func TestDiffLines_RightEmpty(t *testing.T) {
	t.Parallel()
	rows := DiffLines("x\ny\n", "")
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	for _, r := range rows {
		if r.Op != DiffRemoved {
			t.Errorf("op=%v, want Removed", r.Op)
		}
	}
}
