package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/aether/internal/storage"
)

func TestStorage_InitAndPragmas(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// Check WAL mode (AC-S1-04)
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("Expected PRAGMA journal_mode=wal, got %s", journalMode)
	}

	// Double migrations should not cause an error (AC-S1-05)
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("Failed first RunMigrations: %v", err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("Failed second RunMigrations (expected no error): %v", err)
	}
}

func TestStorage_MessageRepository(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("Migrations failed: %v", err)
	}

	repo := storage.NewMessageRepository(db)
	ctx := context.Background()

	msg1 := &storage.Message{
		ID:              "msg-1",
		ConversationID:  "conv-A",
		SenderID:        "peer-A",
		Content:         []byte("hello"),
		GlobalSeq:       1,
		SenderSignature: []byte("sig1"),
		SentAt:          time.Now().UnixMilli(),
	}

	// AC-S1-06: Save + GetSince
	if err := repo.Save(ctx, msg1); err != nil {
		t.Fatalf("Save message failed: %v", err)
	}

	msgs, err := repo.GetSince(ctx, "conv-A", 0, 10)
	if err != nil {
		t.Fatalf("GetSince failed: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "msg-1" {
		t.Fatalf("Expected 1 message with ID msg-1, got %v", msgs)
	}

	// AC-S1-07: Idempotency with INSERT OR IGNORE
	if err := repo.Save(ctx, msg1); err != nil {
		t.Fatalf("Save duplicate message failed: %v", err)
	}
	
	countMsgs, _ := repo.GetSince(ctx, "conv-A", 0, 10)
	if len(countMsgs) != 1 {
		t.Fatalf("Expected exactly 1 message after duplicate save, got %v", len(countMsgs))
	}
}

func TestStorage_DeviceSyncRepository(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()
	storage.RunMigrations(db)

	repo := storage.NewDeviceSyncRepository(db)
	ctx := context.Background()

	devID := "dev-1"
	
	// Default should be 0
	seq, err := repo.GetLastSeq(ctx, devID)
	if err != nil {
		t.Fatalf("GetLastSeq failed: %v", err)
	}
	if seq != 0 {
		t.Fatalf("Expected initial sequence 0, got %d", seq)
	}

	// AC-S1-08: UpdateLastSeq + GetLastSeq
	if err := repo.UpdateLastSeq(ctx, devID, 42, time.Now().UnixMilli()); err != nil {
		t.Fatalf("UpdateLastSeq failed: %v", err)
	}

	seq, err = repo.GetLastSeq(ctx, devID)
	if err != nil {
		t.Fatalf("GetLastSeq failed: %v", err)
	}
	if seq != 42 {
		t.Fatalf("Expected sequence 42, got %d", seq)
	}
}
