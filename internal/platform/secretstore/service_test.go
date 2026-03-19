package secretstore

import "testing"

func TestUpsertAndResolve(t *testing.T) {
	svc := NewService()

	secret, err := svc.Upsert("google_client_secret", "", "s3cr3t")
	if err != nil {
		t.Fatalf("upsert secret failed: %v", err)
	}
	if secret.Ref == "" || secret.Status != "active" {
		t.Fatalf("unexpected secret after create: %+v", secret)
	}
	value, ok := svc.Resolve(secret.Ref)
	if !ok || value != "s3cr3t" {
		t.Fatalf("expected secret resolve, got ok=%v value=%q", ok, value)
	}

	updated, err := svc.Upsert("google_client_secret", secret.Ref, "rotated")
	if err != nil {
		t.Fatalf("update secret failed: %v", err)
	}
	if !updated.CreatedAt.Equal(secret.CreatedAt) {
		t.Fatalf("expected created_at preserved, got old=%s new=%s", secret.CreatedAt, updated.CreatedAt)
	}
	value, ok = svc.Resolve(secret.Ref)
	if !ok || value != "rotated" {
		t.Fatalf("expected rotated secret resolve, got ok=%v value=%q", ok, value)
	}
}

func TestResolveRejectsInactiveAndNilService(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(repo)
	secret, err := svc.Upsert("api_key", "secret:test", "value")
	if err != nil {
		t.Fatalf("upsert secret failed: %v", err)
	}
	secret.Status = "disabled"
	if err := repo.Save(secret); err != nil {
		t.Fatalf("save disabled secret failed: %v", err)
	}
	if _, ok := svc.Resolve(secret.Ref); ok {
		t.Fatal("expected inactive secret to be unresolved")
	}

	var nilSvc *Service
	if items := nilSvc.List(); items != nil {
		t.Fatalf("expected nil service list to be nil, got %+v", items)
	}
	if _, ok := nilSvc.Resolve("secret:test"); ok {
		t.Fatal("expected nil service resolve to fail")
	}
}

func TestListReturnsStoredSecrets(t *testing.T) {
	svc := NewService()
	first, err := svc.Upsert("first", "", "one")
	if err != nil {
		t.Fatalf("upsert first secret failed: %v", err)
	}
	second, err := svc.Upsert("second", "", "two")
	if err != nil {
		t.Fatalf("upsert second secret failed: %v", err)
	}

	items := svc.List()
	if len(items) != 2 {
		t.Fatalf("expected 2 secrets, got %+v", items)
	}
	if items[0].Ref != first.Ref && items[1].Ref != first.Ref {
		t.Fatalf("expected first secret in list, got %+v", items)
	}
	if items[0].Ref != second.Ref && items[1].Ref != second.Ref {
		t.Fatalf("expected second secret in list, got %+v", items)
	}
}
