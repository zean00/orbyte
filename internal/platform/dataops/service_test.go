package dataops

import (
	"context"
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/reference"
)

func TestBackupAndRestoreByDataClassSeparately(t *testing.T) {
	repo := NewMemoryRepository()
	source, stop := newAsyncTestService(repo)
	defer stop()
	if err := source.config.Save(config.Entry{Key: "identity.auth", Scope: "deployment", Value: map[string]any{"password_min_length": 12}, UpdatedBy: "tester"}); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := source.reference.RegisterType(reference.TypeDefinition{Key: "department", DisplayName: "Department"}); err != nil {
		t.Fatalf("register reference type failed: %v", err)
	}
	if err := source.reference.UpsertRecord(reference.Record{TypeKey: "department", Key: "finance", DisplayName: "Finance", Scope: "deployment", UpdatedBy: "tester"}); err != nil {
		t.Fatalf("save reference record failed: %v", err)
	}
	doc, err := source.documents.Create("generic_request", "org_1", "loc_1", "tester", map[string]any{"subject": "demo"})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	run, _, err := source.BackupRun(BackupRequest{
		Name:                "full",
		SelectedDataClasses: []DataClass{DataClassConfiguration, DataClassMaster, DataClassTransactional},
		ActorID:             "tester",
	})
	if err != nil {
		t.Fatalf("backup run failed: %v", err)
	}
	run = waitForOperation(t, source, run.ID)

	target, targetStop := newAsyncTestService(repo)
	defer targetStop()
	restoreRun, _, err := target.RestoreRun(RestoreRequest{ArtifactID: run.ArtifactID, SelectedDataClasses: []DataClass{DataClassConfiguration}, ActorID: "restorer"})
	if err != nil {
		t.Fatalf("configuration restore failed: %v", err)
	}
	waitForOperation(t, target, restoreRun.ID)
	if _, ok := target.config.Get("identity.auth"); !ok {
		t.Fatal("expected configuration entry after configuration-only restore")
	}
	if len(target.reference.Records("department")) != 0 {
		t.Fatal("did not expect reference records after configuration-only restore")
	}
	if _, err := target.documents.Get(doc.Header.ID); err == nil {
		t.Fatal("did not expect document after configuration-only restore")
	}

	restoreRun, _, err = target.RestoreRun(RestoreRequest{ArtifactID: run.ArtifactID, SelectedDataClasses: []DataClass{DataClassMaster}, ActorID: "restorer"})
	if err != nil {
		t.Fatalf("master restore failed: %v", err)
	}
	waitForOperation(t, target, restoreRun.ID)
	if len(target.reference.Records("department")) != 1 {
		t.Fatal("expected reference record after master-only restore")
	}

	restoreRun, _, err = target.RestoreRun(RestoreRequest{ArtifactID: run.ArtifactID, SelectedDataClasses: []DataClass{DataClassTransactional}, ActorID: "restorer"})
	if err != nil {
		t.Fatalf("transactional restore failed: %v", err)
	}
	waitForOperation(t, target, restoreRun.ID)
	if _, err := target.documents.Get(doc.Header.ID); err != nil {
		t.Fatalf("expected document after transactional-only restore: %v", err)
	}
}

func TestArchiveTransactionalOnly(t *testing.T) {
	svc, stop := newAsyncTestService(NewMemoryRepository())
	defer stop()
	if _, err := svc.documents.Create("generic_request", "org_1", "loc_1", "tester", map[string]any{"subject": "archive"}); err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	if _, err := svc.ArchivePlan(ArchiveRequest{SelectedDataClasses: []DataClass{DataClassConfiguration}}); err == nil {
		t.Fatal("expected archive planning for configuration data to fail")
	}
	run, _, err := svc.ArchiveRun(ArchiveRequest{SelectedDataClasses: []DataClass{DataClassTransactional}, ActorID: "tester"})
	if err != nil {
		t.Fatalf("archive run failed: %v", err)
	}
	run = waitForOperation(t, svc, run.ID)
	artifact, ok := svc.Artifact(run.ArtifactID)
	if !ok {
		t.Fatal("expected archive artifact after async archive")
	}
	if artifact.Type != ArtifactTypeArchive {
		t.Fatalf("expected archive artifact, got %s", artifact.Type)
	}
	if len(svc.documents.List()) != 0 {
		t.Fatal("expected live transactional records to be removed after archive")
	}
}

func TestMigrationValidationAndRun(t *testing.T) {
	svc, stop := newAsyncTestService(NewMemoryRepository())
	defer stop()
	artifact, err := svc.MigrationRegister(MigrationRegisterRequest{
		Name:                "migration",
		SelectedDataClasses: []DataClass{DataClassTransactional},
		Segments: []MigrationSegment{{
			DataClass:  DataClassTransactional,
			AdapterKey: "documents.records",
			Records: []any{
				map[string]any{"organization_id": "org_1", "location_id": "loc_1", "payload": map[string]any{"subject": "missing type"}},
			},
		}},
		ActorID: "tester",
	})
	if err != nil {
		t.Fatalf("register migration failed: %v", err)
	}
	plan, err := svc.MigrationPlan(RestoreRequest{ArtifactID: artifact.ID, SelectedDataClasses: []DataClass{DataClassTransactional}})
	if err != nil {
		t.Fatalf("migration plan failed: %v", err)
	}
	if plan.Validation.Valid {
		t.Fatal("expected invalid migration plan for malformed document input")
	}

	artifact, err = svc.MigrationRegister(MigrationRegisterRequest{
		Name:                "valid-migration",
		SelectedDataClasses: []DataClass{DataClassTransactional},
		Segments: []MigrationSegment{{
			DataClass:  DataClassTransactional,
			AdapterKey: "documents.records",
			Records: []any{
				map[string]any{"document_type": "generic_request", "organization_id": "org_1", "location_id": "loc_1", "actor_id": "tester", "payload": map[string]any{"subject": "valid"}},
			},
		}},
		ActorID: "tester",
	})
	if err != nil {
		t.Fatalf("register valid migration failed: %v", err)
	}
	run, _, err := svc.MigrationRun(RestoreRequest{ArtifactID: artifact.ID, SelectedDataClasses: []DataClass{DataClassTransactional}, ActorID: "tester"})
	if err != nil {
		t.Fatalf("migration run failed: %v", err)
	}
	waitForOperation(t, svc, run.ID)
	if len(svc.documents.List()) != 1 {
		t.Fatalf("expected migrated document, got %d", len(svc.documents.List()))
	}
}

func TestIncrementalBackupRestoreIncludesBaseArtifactData(t *testing.T) {
	repo := NewMemoryRepository()
	source, stop := newAsyncTestService(repo)
	defer stop()
	if err := source.config.Save(config.Entry{Key: "identity.auth", Scope: "deployment", Value: map[string]any{"password_min_length": 10}, UpdatedBy: "tester"}); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := source.reference.RegisterType(reference.TypeDefinition{Key: "department", DisplayName: "Department"}); err != nil {
		t.Fatalf("register reference type failed: %v", err)
	}
	if err := source.reference.UpsertRecord(reference.Record{TypeKey: "department", Key: "finance", DisplayName: "Finance", Scope: "deployment", UpdatedBy: "tester"}); err != nil {
		t.Fatalf("save reference record failed: %v", err)
	}
	fullRun, _, err := source.BackupRun(BackupRequest{
		Name:                "full",
		SelectedDataClasses: []DataClass{DataClassConfiguration, DataClassMaster},
		ActorID:             "tester",
	})
	if err != nil {
		t.Fatalf("full backup run failed: %v", err)
	}
	fullRun = waitForOperation(t, source, fullRun.ID)
	if err := source.config.Save(config.Entry{Key: "identity.auth", Scope: "deployment", Value: map[string]any{"password_min_length": 12}, UpdatedBy: "tester"}); err != nil {
		t.Fatalf("update config failed: %v", err)
	}
	incrementalRun, _, err := source.BackupRun(BackupRequest{
		Name:                "incremental",
		SelectedDataClasses: []DataClass{DataClassConfiguration, DataClassMaster},
		Incremental:         true,
		ActorID:             "tester",
	})
	if err != nil {
		t.Fatalf("incremental backup run failed: %v", err)
	}
	incrementalRun = waitForOperation(t, source, incrementalRun.ID)
	incrementalArtifact, ok := source.Artifact(incrementalRun.ArtifactID)
	if !ok {
		t.Fatal("expected incremental artifact")
	}
	if incrementalArtifact.Manifest.BaseArtifactID != fullRun.ArtifactID {
		t.Fatalf("expected incremental backup to link to base artifact %s, got %s", fullRun.ArtifactID, incrementalArtifact.Manifest.BaseArtifactID)
	}

	target, targetStop := newAsyncTestService(repo)
	defer targetStop()
	restoreRun, _, err := target.RestoreRun(RestoreRequest{
		ArtifactID:          incrementalRun.ArtifactID,
		SelectedDataClasses: []DataClass{DataClassConfiguration, DataClassMaster},
		ActorID:             "restorer",
	})
	if err != nil {
		t.Fatalf("incremental restore failed: %v", err)
	}
	waitForOperation(t, target, restoreRun.ID)
	entry, ok := target.config.Get("identity.auth")
	if !ok {
		t.Fatal("expected configuration entry after incremental restore")
	}
	value := entry.Value
	got := 0
	switch typed := value["password_min_length"].(type) {
	case int:
		got = typed
	case float64:
		got = int(typed)
	default:
		t.Fatalf("expected numeric password_min_length, got %#v", value["password_min_length"])
	}
	if got != 12 {
		t.Fatalf("expected updated config value 12 after incremental restore, got %d", got)
	}
	if len(target.reference.Records("department")) != 1 {
		t.Fatal("expected unchanged master data restored from base artifact chain")
	}
}

func TestMigrationValidationRejectsUnsupportedAdapter(t *testing.T) {
	svc, stop := newAsyncTestService(NewMemoryRepository())
	defer stop()
	artifact, err := svc.MigrationRegister(MigrationRegisterRequest{
		Name:                "unsupported-adapter",
		SelectedDataClasses: []DataClass{DataClassConfiguration},
		Segments: []MigrationSegment{{
			DataClass:  DataClassConfiguration,
			AdapterKey: "unknown.adapter",
			Records:    []any{map[string]any{"key": "value"}},
		}},
		ActorID: "tester",
	})
	if err != nil {
		t.Fatalf("register migration failed: %v", err)
	}
	plan, err := svc.MigrationPlan(RestoreRequest{ArtifactID: artifact.ID, SelectedDataClasses: []DataClass{DataClassConfiguration}})
	if err != nil {
		t.Fatalf("migration plan failed: %v", err)
	}
	if plan.Validation.Valid {
		t.Fatal("expected invalid migration plan for unsupported adapter")
	}
}

func newAsyncTestService(repo Repository) (*Service, func()) {
	cfg := config.NewService()
	flags := featureflags.NewService()
	modules := module.NewService()
	_ = modules.Register(module.Manifest{Key: "platform.core", Name: "Platform Core", Version: "1.0.0", DomainFamily: "platform"}, "system")
	referenceSvc := reference.NewService()
	identitySvc := identity.NewService(nil)
	documentSvc := document.NewService()
	integrationSvc := integration.NewService(nil, nil)
	svc := NewServiceWithRepository(repo, cfg, flags, modules, referenceSvc, identitySvc, documentSvc, integrationSvc)
	jobSvc := jobs.NewService()
	svc.AttachJobs(jobSvc)
	ctx, cancel := context.WithCancel(context.Background())
	jobSvc.Start(ctx)
	return svc, func() {
		cancel()
		jobSvc.Stop()
	}
}

func waitForOperation(t *testing.T, svc *Service, operationID string) OperationRun {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, ok := svc.Operation(operationID)
		if ok && (run.Status == jobs.StatusSucceeded || run.Status == jobs.StatusFailed || run.Status == jobs.StatusDeadLetter) {
			if run.Status != jobs.StatusSucceeded {
				t.Fatalf("expected succeeded dataops operation, got %+v", run)
			}
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for operation %s", operationID)
	return OperationRun{}
}
