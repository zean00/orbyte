package httpx

import (
	"sync"
	"testing"
	"time"

	"orbyte/internal/platform/module"
)

func TestWorkspaceBootstrapCacheKeyIncludesDeepLinkGrantID(t *testing.T) {
	base := principal{
		userID:            "user-1",
		effectiveUserID:   "user-1",
		currentLocationID: "loc-1",
	}
	left := base
	left.deepLinkGrantID = "dl-1"
	right := base
	right.deepLinkGrantID = "dl-2"

	leftKey := workspaceBootstrapCacheKey(left, module.UISurfaceBackoffice, "en")
	rightKey := workspaceBootstrapCacheKey(right, module.UISurfaceBackoffice, "en")
	if leftKey == rightKey {
		t.Fatalf("expected deep-link grant id to affect cache key, got %q", leftKey)
	}
}

func TestSweepExpiredWorkspaceBootstrapEntriesDeletesExpiredEntries(t *testing.T) {
	workspaceBootstrapCache = sync.Map{}
	t.Cleanup(func() {
		workspaceBootstrapCache = sync.Map{}
		workspaceBootstrapCacheSweep.mu.Lock()
		workspaceBootstrapCacheSweep.lastScan = time.Time{}
		workspaceBootstrapCacheSweep.mu.Unlock()
	})
	now := time.Now()
	workspaceBootstrapCache.Store("expired", cachedWorkspaceBootstrap{
		payload:   map[string]any{"shell_kind": "workspace"},
		expiresAt: now.Add(-time.Second),
	})
	workspaceBootstrapCache.Store("fresh", cachedWorkspaceBootstrap{
		payload:   map[string]any{"shell_kind": "workspace"},
		expiresAt: now.Add(time.Second),
	})
	workspaceBootstrapCacheSweep.mu.Lock()
	workspaceBootstrapCacheSweep.lastScan = time.Time{}
	workspaceBootstrapCacheSweep.mu.Unlock()

	sweepExpiredWorkspaceBootstrapEntries(now)

	if _, ok := workspaceBootstrapCache.Load("expired"); ok {
		t.Fatal("expected expired cache entry to be evicted")
	}
	if _, ok := workspaceBootstrapCache.Load("fresh"); !ok {
		t.Fatal("expected fresh cache entry to remain")
	}
}
