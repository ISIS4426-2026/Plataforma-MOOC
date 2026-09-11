package domain_test

import (
	"reflect"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

func TestInsertAtInsertsAtClampedIndex(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		newID    string
		index    int
		want     []string
	}{
		{"middle", []string{"a", "b", "c"}, "x", 1, []string{"a", "x", "b", "c"}},
		{"start", []string{"a", "b", "c"}, "x", 0, []string{"x", "a", "b", "c"}},
		{"end", []string{"a", "b", "c"}, "x", 3, []string{"a", "b", "c", "x"}},
		{"negative index clamps to start", []string{"a", "b"}, "x", -5, []string{"x", "a", "b"}},
		{"past-end index clamps to end", []string{"a", "b"}, "x", 99, []string{"a", "b", "x"}},
		{"empty container", []string{}, "x", 0, []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.InsertAt(tt.existing, tt.newID, tt.index)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InsertAt(%v, %q, %d) = %v, want %v", tt.existing, tt.newID, tt.index, got, tt.want)
			}
		})
	}
}

func TestMoveToRelocatesWithoutDuplicating(t *testing.T) {
	existing := []string{"a", "b", "c", "d"}

	got := domain.MoveTo(existing, "d", 1)
	want := []string{"a", "d", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MoveTo(%v, %q, %d) = %v, want %v", existing, "d", 1, got, want)
	}

	for _, id := range existing {
		count := 0
		for _, g := range got {
			if g == id {
				count++
			}
		}
		if count != 1 {
			t.Errorf("id %q appears %d times after the move, want exactly 1", id, count)
		}
	}
}

func TestMoveToUnknownIDIsANoOp(t *testing.T) {
	existing := []string{"a", "b", "c"}

	got := domain.MoveTo(existing, "not-a-sibling", 0)

	if !reflect.DeepEqual(got, existing) {
		t.Errorf("MoveTo with an unknown id = %v, want the order unchanged (%v)", got, existing)
	}
}

func TestRepositionAssignsContiguousZeroBasedPositions(t *testing.T) {
	order := []string{"b", "d", "a", "c"}

	got := domain.Reposition(order)

	want := map[string]int{"b": 0, "d": 1, "a": 2, "c": 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Reposition(%v) = %v, want %v", order, got, want)
	}
}

// TestReorderNeverCollides is a property check standing in for "no
// collisions ever": for every index a sibling can move to, in a container of
// realistic size, the resulting position map assigns each stable_id a
// distinct position and every position in [0, n) is used exactly once.
func TestReorderNeverCollides(t *testing.T) {
	base := []string{"m1", "m2", "m3", "m4", "m5"}

	for movingIndex := range base {
		for target := 0; target <= len(base); target++ {
			order := domain.MoveTo(base, base[movingIndex], target)
			positions := domain.Reposition(order)

			if len(positions) != len(base) {
				t.Fatalf("moving index %d to %d: got %d positions, want %d", movingIndex, target, len(positions), len(base))
			}

			seen := make([]bool, len(base))
			for _, pos := range positions {
				if pos < 0 || pos >= len(base) {
					t.Fatalf("moving index %d to %d: position %d out of range", movingIndex, target, pos)
				}
				if seen[pos] {
					t.Fatalf("moving index %d to %d: position %d assigned to two stable IDs", movingIndex, target, pos)
				}
				seen[pos] = true
			}
		}
	}
}

// TestReorderPreservesStableIdentityForProgress is the acceptance test asked
// for by issue #16: reordering the resources within a unit must not change
// any stable_id, so a StudentProgress record that names a resource by its
// stable_id keeps pointing at the same resource after the reorder as before
// it — only its Position field, never its identity, moves.
func TestReorderPreservesStableIdentityForProgress(t *testing.T) {
	unit := &domain.Unit{
		ID:       "unit-row-1",
		StableID: "unit-stable-1",
		Resources: []domain.Resource{
			{ID: "r1-row", StableID: "r1-stable", Title: "Intro", Position: 0},
			{ID: "r2-row", StableID: "r2-stable", Title: "Reading", Position: 1},
			{ID: "r3-row", StableID: "r3-stable", Title: "Quiz", Position: 2},
		},
	}

	// A student completed the third resource before the reorder, recorded
	// exclusively by its stable_id, as issue #16 and #20 require.
	progress := domain.StudentProgress{
		StudentID:          "student-1",
		CourseStableID:     "course-stable-1",
		CompletedResources: []string{"r3-stable"},
	}

	// The professor moves that same resource ("Quiz") to the front of the
	// unit. This is an in-place reorder: same stable_ids, new positions.
	order := currentOrder(unit.Resources)
	order = domain.MoveTo(order, "r3-stable", 0)
	newPositions := domain.Reposition(order)
	applyPositions(unit.Resources, newPositions)

	// The resource the student completed is still found by its stable_id,
	// and none of the stable_ids changed — only where each one sits.
	completed := findByStableID(unit.Resources, progress.CompletedResources[0])
	if completed == nil {
		t.Fatal("resource completed by the student can no longer be found by its stable_id after reordering")
	}
	if completed.Title != "Quiz" {
		t.Errorf("stable_id %q now resolves to %q, want the same resource (\"Quiz\") as before the reorder",
			progress.CompletedResources[0], completed.Title)
	}
	if completed.Position != 0 {
		t.Errorf("Quiz should have moved to position 0, got %d", completed.Position)
	}

	gotStableIDs := make(map[string]bool, len(unit.Resources))
	for _, r := range unit.Resources {
		gotStableIDs[r.StableID] = true
	}
	for _, want := range []string{"r1-stable", "r2-stable", "r3-stable"} {
		if !gotStableIDs[want] {
			t.Errorf("stable_id %q disappeared after reordering", want)
		}
	}
}

func currentOrder(resources []domain.Resource) []string {
	order := make([]string, len(resources))
	for i, r := range resources {
		order[i] = r.StableID
	}
	return order
}

func applyPositions(resources []domain.Resource, positions map[string]int) {
	for i := range resources {
		resources[i].Position = positions[resources[i].StableID]
	}
}

func findByStableID(resources []domain.Resource, stableID string) *domain.Resource {
	for i := range resources {
		if resources[i].StableID == stableID {
			return &resources[i]
		}
	}
	return nil
}
