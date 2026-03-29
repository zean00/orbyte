package document

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemoryRepositorySupportsConcurrentRecordAccess(t *testing.T) {
	repo := NewMemoryRepository()
	record := Record{
		Header: Header{
			ID:           "doc-concurrent",
			Type:         "generic_request",
			Status:       "draft",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
			OrganizationID: "org_default",
			LocationID:   "loc_hq",
		},
	}
	if err := repo.SaveRecord(record); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				next := record
				next.Header.Status = fmt.Sprintf("draft-%d", worker)
				next.Header.UpdatedAt = time.Now().UTC()
				if err := repo.SaveLines(record.Header.ID, []Line{{LineNo: 1, LineType: "item", Payload: map[string]any{"worker": worker, "iteration": j}}}); err != nil {
					t.Errorf("save lines: %v", err)
					return
				}
				if err := repo.SaveRecord(next); err != nil {
					t.Errorf("save record: %v", err)
					return
				}
				if _, ok := repo.GetRecord(record.Header.ID); !ok {
					t.Error("expected record to exist")
					return
				}
				_ = repo.ListRecords()
			}
		}(i)
	}
	wg.Wait()

	loaded, ok := repo.GetRecord(record.Header.ID)
	if !ok {
		t.Fatal("expected saved record after concurrent access")
	}
	if loaded.Header.ID != record.Header.ID {
		t.Fatalf("expected document id %s, got %s", record.Header.ID, loaded.Header.ID)
	}
}
