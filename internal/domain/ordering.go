package domain

// Position recalculation for the academic hierarchy (issue #16).
//
// Modules within a course, units within a module, and resources within a
// unit each form a sibling set with two identifiers per item: a row id that
// changes every time a new course version is published, and a stable_id that
// never does. Ordering must never be expressed through the stable_id itself
// (e.g. by encoding order into it) because that would make reordering
// indistinguishable from renaming; it is expressed entirely through the
// separate Position field, which is free to change on every write.
//
// # How positions are recalculated
//
// On any insert or move, every sibling in the affected container is
// renumbered in one pass: the desired order is expressed as the full,
// deduplicated list of sibling stable IDs, and Reposition walks it assigning
// 0, 1, 2, ... in order. Because the output always covers the entire sibling
// set in a single pass, no two siblings can ever be assigned the same
// position — there is no code path that could produce a duplicate, rather
// than a check that catches one after the fact.
//
// InsertAt and MoveTo are the two callers that build that desired order:
// InsertAt splices a new stable_id in at a clamped index (used when a module,
// unit or resource is created), and MoveTo relocates an existing one (used
// when authoring reorders the list). Both leave every other stable_id's
// relative order untouched, so a move or insert only ever shifts the
// positions of the siblings between the old and new spot, never their
// identity.
//
// The repository applying the resulting positions writes them inside one
// transaction against a DEFERRABLE UNIQUE (parent_id, position) constraint
// (migrations/000003_academic_ordering.up.sql). Deferring the check to commit
// is what makes the rewrite safe even though individual UPDATE statements
// within the transaction can transiently collide with a position that has
// not been overwritten yet: Postgres only rejects the transaction if a
// collision still exists once every row in the pass has been written.

// InsertAt returns the sibling order that results from inserting newID into
// existing at index, shifting the items at and after index one place later.
//
// index is clamped to [0, len(existing)], so an out-of-range caller appends
// or prepends instead of panicking. If newID already appears in existing, the
// prior occurrence is removed first so the id is never listed twice.
func InsertAt(existing []string, newID string, index int) []string {
	without := removeID(existing, newID)

	if index < 0 {
		index = 0
	}
	if index > len(without) {
		index = len(without)
	}

	result := make([]string, 0, len(without)+1)
	result = append(result, without[:index]...)
	result = append(result, newID)
	result = append(result, without[index:]...)
	return result
}

// MoveTo returns the sibling order that results from moving id to index
// within the same container. Moving an id not present in existing is a
// no-op: the order is returned unchanged.
func MoveTo(existing []string, id string, index int) []string {
	if !contains(existing, id) {
		return existing
	}
	return InsertAt(existing, id, index)
}

// Reposition assigns contiguous, zero-based positions to a full sibling set,
// in the order given. The caller is expected to have already produced that
// order with InsertAt or MoveTo (or to be restating the current order
// unchanged); Reposition itself has no notion of "before" or "after", only of
// the sequence it is handed.
func Reposition(orderedStableIDs []string) map[string]int {
	positions := make(map[string]int, len(orderedStableIDs))
	for i, id := range orderedStableIDs {
		positions[id] = i
	}
	return positions
}

func removeID(ids []string, target string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != target {
			result = append(result, id)
		}
	}
	return result
}

func contains(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
