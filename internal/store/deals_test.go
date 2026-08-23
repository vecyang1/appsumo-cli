package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/store"
)

func intPtr(value int) *int { return &value }

// TestDealSnapshotRoundTripKeepsUnknownsUnknown is the persistence half of the
// "absent is not zero" guard. appsumo.Deal carefully leaves CodesRemaining nil
// when the catalog did not state a stock count; a store that writes nil as 0
// puts the confident-wrong-number back after one round trip, and the next diff
// then reports a phantom "unknown -> 0" move on hundreds of deals.
func TestDealSnapshotRoundTripKeepsUnknownsUnknown(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	stamp, err := db.SaveDealSnapshot(ctx, time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC), []appsumo.Deal{
		{Slug: "no-codes", Name: "No Codes", Price: 69, OriginalPrice: 1999},
		{Slug: "sold-out", Name: "Sold Out", Price: 49, CodesRemaining: intPtr(0)},
		{Slug: "stocked", Name: "Stocked", Price: 29, CodesRemaining: intPtr(1126)},
	})
	if err != nil {
		t.Fatalf("SaveDealSnapshot returned error: %v", err)
	}

	loaded, err := db.LoadSnapshot(ctx, stamp)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("loaded %d deals, want 3", len(loaded))
	}

	byslug := map[string]appsumo.Deal{}
	for _, deal := range loaded {
		byslug[deal.Slug] = deal
	}
	if byslug["no-codes"].CodesRemaining != nil {
		t.Fatalf("an unknown stock count came back as %d", *byslug["no-codes"].CodesRemaining)
	}
	if got := byslug["sold-out"].CodesRemaining; got == nil || *got != 0 {
		t.Fatalf("a genuine zero came back as %v", got)
	}
	if got := byslug["stocked"].CodesRemaining; got == nil || *got != 1126 {
		t.Fatalf("a real stock count came back as %v", got)
	}
	if byslug["no-codes"].Price != 69 || byslug["no-codes"].OriginalPrice != 1999 {
		t.Fatalf("prices did not survive the round trip: %#v", byslug["no-codes"])
	}

	// The round trip must not manufacture a diff against the values that went in.
	if changes := appsumo.DiffDeals(loaded, loaded); len(changes) != 0 {
		t.Fatalf("a snapshot differs from itself: %#v", changes)
	}
}

func TestDealSnapshotsAreOrderedAndDiffable(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	earlier := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)

	if _, err := db.SaveDealSnapshot(ctx, earlier, []appsumo.Deal{
		{Slug: "keeper", Name: "Keeper", Price: 99},
		{Slug: "leaver", Name: "Leaver", Price: 39},
	}); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if _, err := db.SaveDealSnapshot(ctx, later, []appsumo.Deal{
		{Slug: "keeper", Name: "Keeper", Price: 69},
		{Slug: "joiner", Name: "Joiner", Price: 59},
	}); err != nil {
		t.Fatalf("second snapshot: %v", err)
	}

	stamps, err := db.SnapshotIDs(ctx, 2)
	if err != nil {
		t.Fatalf("SnapshotIDs returned error: %v", err)
	}
	if len(stamps) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(stamps))
	}
	if stamps[0] != later.UTC().Format("2006-01-02T15:04:05.000000000Z") {
		t.Fatalf("snapshots are not newest-first: %v", stamps)
	}

	before, err := db.LoadSnapshot(ctx, stamps[1])
	if err != nil {
		t.Fatalf("LoadSnapshot(before): %v", err)
	}
	after, err := db.LoadSnapshot(ctx, stamps[0])
	if err != nil {
		t.Fatalf("LoadSnapshot(after): %v", err)
	}

	kinds := map[string]string{}
	for _, change := range appsumo.DiffDeals(before, after) {
		kinds[change.Slug] = change.Kind
	}
	if kinds["joiner"] != "new" || kinds["leaver"] != "gone" || kinds["keeper"] != "changed" {
		t.Fatalf("diff over stored snapshots wrong: %#v", kinds)
	}
}

func TestPruneSnapshotsKeepsTheNewest(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for day := 0; day < 5; day++ {
		if _, err := db.SaveDealSnapshot(ctx, base.AddDate(0, 0, day), []appsumo.Deal{
			{Slug: "one", Name: "One", Price: float64(10 + day)},
		}); err != nil {
			t.Fatalf("snapshot %d: %v", day, err)
		}
	}
	if _, err := db.PruneSnapshots(ctx, 2); err != nil {
		t.Fatalf("PruneSnapshots returned error: %v", err)
	}
	stamps, err := db.SnapshotIDs(ctx, 10)
	if err != nil {
		t.Fatalf("SnapshotIDs returned error: %v", err)
	}
	if len(stamps) != 2 {
		t.Fatalf("kept %d snapshots, want 2: %v", len(stamps), stamps)
	}
	if stamps[0] != base.AddDate(0, 0, 4).UTC().Format("2006-01-02T15:04:05.000000000Z") {
		t.Fatalf("pruning kept the wrong snapshots: %v", stamps)
	}
	if _, err := db.PruneSnapshots(ctx, 0); err == nil {
		t.Fatal("PruneSnapshots(0) would delete everything and was accepted")
	}
}

func TestSaveReviewsAndQuestionsFlattenThreads(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	reviews, err := decodeReviews(`[{
		"id": 1, "deal_id": 257731, "rating": 5, "title": "Great", "comment": "Body",
		"user": {"id": 9, "username": "buyer"},
		"children": [{"id": 2, "deal_id": 257731, "parent_id": 1, "level": 1,
			"comment": "Thanks!", "rating": null, "user": {"id": 10, "username": "founder"}}]
	}]`)
	if err != nil {
		t.Fatalf("decode reviews fixture: %v", err)
	}
	saved, err := db.SaveReviews(ctx, "growify", reviews)
	if err != nil {
		t.Fatalf("SaveReviews returned error: %v", err)
	}
	if saved != 2 {
		t.Fatalf("stored %d rows, want 2 (review plus reply)", saved)
	}

	questions, err := decodeQuestions(`[{
		"id": 100, "deal_id": 213645, "comment": "Agency use?",
		"user": {"id": 11, "username": "asker"},
		"children": [{"id": 101, "deal_id": 213645, "parent_id": 100, "level": 1,
			"comment": "Yes.", "user": {"id": 10, "username": "founder"}}]
	}, {
		"id": 200, "deal_id": 213645, "comment": "No answer yet?",
		"user": {"id": 12, "username": "waiting"}, "children": []
	}]`)
	if err != nil {
		t.Fatalf("decode questions fixture: %v", err)
	}
	if _, err := db.SaveQuestions(ctx, "growify", questions); err != nil {
		t.Fatalf("SaveQuestions returned error: %v", err)
	}

	reviewCount, err := db.CountComments(ctx, "growify", store.KindReview)
	if err != nil {
		t.Fatalf("CountComments(review): %v", err)
	}
	questionCount, err := db.CountComments(ctx, "growify", store.KindQuestion)
	if err != nil {
		t.Fatalf("CountComments(question): %v", err)
	}
	if reviewCount != 2 || questionCount != 3 {
		t.Fatalf("counts wrong: reviews=%d questions=%d", reviewCount, questionCount)
	}

	// A reply carries no rating, and neither does a question. Storing 0 there
	// would drag every average down by inventing one-star rows.
	rows, err := db.QueryReadOnly(ctx, `select kind, id, rating, answered from product_comments
		where rating is null order by kind, id`)
	if err != nil {
		t.Fatalf("QueryReadOnly returned error: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows with no rating (1 reply + 3 question rows), got %d: %#v", len(rows), rows)
	}

	answered, err := db.QueryReadOnly(ctx, `select id from product_comments
		where kind = 'question' and answered = 1 order by id`)
	if err != nil {
		t.Fatalf("QueryReadOnly(answered) returned error: %v", err)
	}
	if len(answered) != 1 || answered[0]["id"] != "100" {
		t.Fatalf("answered flag wrong: %#v", answered)
	}

	// Re-saving the same crawl must not duplicate rows.
	if _, err := db.SaveReviews(ctx, "growify", reviews); err != nil {
		t.Fatalf("second SaveReviews returned error: %v", err)
	}
	again, err := db.CountComments(ctx, "growify", store.KindReview)
	if err != nil {
		t.Fatalf("CountComments after re-save: %v", err)
	}
	if again != 2 {
		t.Fatalf("re-saving duplicated rows: %d", again)
	}
}

// TestRapidSnapshotsDoNotCollapse pins the id precision. With second-precision
// stamps two syncs inside the same second produced one id, the second walk
// upserted over the first, and `deals diff` then reported that it had only one
// snapshot to compare.
func TestRapidSnapshotsDoNotCollapse(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	moment := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	first, err := db.SaveDealSnapshot(ctx, moment, []appsumo.Deal{{Slug: "one", Price: 49}})
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	second, err := db.SaveDealSnapshot(ctx, moment.Add(3*time.Millisecond), []appsumo.Deal{{Slug: "one", Price: 39}})
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if first == second {
		t.Fatalf("two snapshots 3ms apart share the id %q", first)
	}

	stamps, err := db.SnapshotIDs(ctx, 10)
	if err != nil {
		t.Fatalf("SnapshotIDs returned error: %v", err)
	}
	if len(stamps) != 2 {
		t.Fatalf("kept %d snapshots, want 2: %v", len(stamps), stamps)
	}
	if stamps[0] != second {
		t.Fatalf("sub-second stamps do not sort newest-first: %v", stamps)
	}
}
