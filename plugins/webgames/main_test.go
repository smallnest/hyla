package main

import (
	"testing"
	"time"
)

func TestStoreHighScoresAndBookmarks(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	snap, err := st.RecordScore("2048", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if snap.GamesPlayed != 1 || snap.HighScores["2048"] != 1024 {
		t.Fatalf("unexpected snap after first score: %+v", snap)
	}

	// Lower score does not overwrite high score.
	snap, err = st.RecordScore("2048", 512)
	if err != nil {
		t.Fatal(err)
	}
	if snap.GamesPlayed != 2 || snap.HighScores["2048"] != 1024 {
		t.Fatalf("high score overwritten by lower score: %+v", snap)
	}

	now := time.UnixMilli(1700000000000)
	snap, err = st.AddBookmark("Play2048", "play2048.co", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Bookmarks) != 1 || snap.Bookmarks[0].URL != "https://play2048.co" {
		t.Fatalf("unexpected bookmarks: %+v", snap.Bookmarks)
	}

	// Re-open store from disk to verify persistence.
	st2, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap2 := st2.Snapshot()
	if snap2.HighScores["2048"] != 1024 || len(snap2.Bookmarks) != 1 {
		t.Fatalf("persisted state mismatch: %+v", snap2)
	}

	snap2, err = st2.RemoveBookmark(snap2.Bookmarks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap2.Bookmarks) != 0 {
		t.Fatalf("bookmark was not removed: %+v", snap2.Bookmarks)
	}
}

func TestAutoPauseTransition(t *testing.T) {
	svc := newWebGamesService()
	if _, err := svc.ensureStore(); err != nil {
		t.Fatal(err)
	}

	if svc.shouldAutoPause("p1", "working") {
		t.Fatal("should not pause when transitioning to working")
	}
	if !svc.shouldAutoPause("p1", "idle") {
		t.Fatal("expected autoPause on working -> idle transition")
	}
	if svc.shouldAutoPause("p1", "idle") {
		t.Fatal("should not re-trigger autoPause while staying idle")
	}

	_, _ = svc.store.UpdateSettings(GameSettings{AutoPause: false, DefaultGame: "snake"})
	_ = svc.shouldAutoPause("p1", "working")
	if svc.shouldAutoPause("p1", "idle") {
		t.Fatal("should not autoPause when disabled in settings")
	}
}
