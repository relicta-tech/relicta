package communication

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

func TestBundler_BundleChanges_Empty(t *testing.T) {
	b := NewBundler()

	t.Run("nil changeset", func(t *testing.T) {
		bundles := b.BundleChanges(nil)
		if bundles != nil {
			t.Errorf("expected nil, got %d bundles", len(bundles))
		}
	})

	t.Run("empty changeset", func(t *testing.T) {
		cs := changes.NewChangeSet("empty", "v0.0.0", "HEAD")
		bundles := b.BundleChanges(cs)
		if bundles != nil {
			t.Errorf("expected nil, got %d bundles", len(bundles))
		}
	})
}

func TestBundler_BundleChanges_SingleType(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("aaa", changes.CommitTypeFeat, "add login page",
		changes.WithScope("auth"),
	))
	cs.AddCommit(changes.NewConventionalCommit("bbb", changes.CommitTypeFeat, "add signup flow",
		changes.WithScope("auth"),
	))

	bundles := b.BundleChanges(cs)
	if len(bundles) == 0 {
		t.Fatal("expected at least one bundle")
	}

	found := false
	for _, bundle := range bundles {
		if bundle.Type == BundleTypeFeature {
			found = true
			if len(bundle.Changes) != 2 {
				t.Errorf("feature bundle has %d changes, want 2", len(bundle.Changes))
			}
		}
	}
	if !found {
		t.Error("no feature bundle found")
	}
}

func TestBundler_BundleChanges_BreakingChanges(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("aaa", changes.CommitTypeFeat, "redesign user API",
		changes.WithScope("api"),
		changes.WithBreaking("removed /v1/users endpoint"),
	))
	cs.AddCommit(changes.NewConventionalCommit("bbb", changes.CommitTypeFeat, "add search",
		changes.WithScope("search"),
	))
	cs.AddCommit(changes.NewConventionalCommit("ccc", changes.CommitTypeFix, "fix crash on empty input"))

	bundles := b.BundleChanges(cs)

	var breakingBundles []Bundle
	for _, bundle := range bundles {
		if bundle.Type == BundleTypeBreaking {
			breakingBundles = append(breakingBundles, bundle)
		}
	}

	if len(breakingBundles) == 0 {
		t.Fatal("expected at least one breaking bundle")
	}

	totalBreaking := 0
	for _, bb := range breakingBundles {
		totalBreaking += len(bb.Changes)
	}
	if totalBreaking != 1 {
		t.Errorf("expected 1 breaking change, got %d", totalBreaking)
	}
}

func TestBundler_BundleChanges_MultipleScopes(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("aaa", changes.CommitTypeFix, "fix auth token refresh",
		changes.WithScope("auth"),
	))
	cs.AddCommit(changes.NewConventionalCommit("bbb", changes.CommitTypeFix, "fix payment rounding",
		changes.WithScope("payment"),
	))
	cs.AddCommit(changes.NewConventionalCommit("ccc", changes.CommitTypeFix, "fix cart total",
		changes.WithScope("payment"),
	))

	bundles := b.BundleChanges(cs)

	var bugfixBundles []Bundle
	for _, bundle := range bundles {
		if bundle.Type == BundleTypeBugfix {
			bugfixBundles = append(bugfixBundles, bundle)
		}
	}

	// Should have separate bundles for auth and payment scopes
	if len(bugfixBundles) < 2 {
		t.Errorf("expected at least 2 bugfix bundles (one per scope), got %d", len(bugfixBundles))
	}
}

func TestBundler_BundleChanges_SecurityDetection(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("aaa", changes.CommitTypeFix, "fix XSS vulnerability in comments",
		changes.WithScope("web"),
	))
	cs.AddCommit(changes.NewConventionalCommit("bbb", changes.CommitTypeFix, "fix typo in readme"))

	bundles := b.BundleChanges(cs)

	var securityBundles []Bundle
	for _, bundle := range bundles {
		if bundle.Type == BundleTypeSecurity {
			securityBundles = append(securityBundles, bundle)
		}
	}

	if len(securityBundles) == 0 {
		t.Fatal("expected security bundle for XSS vulnerability fix")
	}

	found := false
	for _, sb := range securityBundles {
		for _, c := range sb.Changes {
			if c.Description == "fix XSS vulnerability in comments" {
				found = true
			}
		}
	}
	if !found {
		t.Error("security bundle should contain the XSS fix")
	}
}

func TestBundler_BundleChanges_AllTypes(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v2.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("a1", changes.CommitTypeFeat, "add dashboard",
		changes.WithBreaking("removed old admin panel"),
	))
	cs.AddCommit(changes.NewConventionalCommit("a2", changes.CommitTypeFeat, "add export feature"))
	cs.AddCommit(changes.NewConventionalCommit("a3", changes.CommitTypeFix, "fix login timeout"))
	cs.AddCommit(changes.NewConventionalCommit("a4", changes.CommitTypePerf, "optimize query performance"))
	cs.AddCommit(changes.NewConventionalCommit("a5", changes.CommitTypeDocs, "update API docs"))
	cs.AddCommit(changes.NewConventionalCommit("a6", changes.CommitTypeChore, "update dependencies"))

	bundles := b.BundleChanges(cs)
	if len(bundles) == 0 {
		t.Fatal("expected bundles")
	}

	typeCount := make(map[BundleType]int)
	for _, bundle := range bundles {
		typeCount[bundle.Type]++
	}

	// Should have at least breaking, feature, bugfix, performance, docs, and chore bundles
	expectedTypes := []BundleType{BundleTypeBreaking, BundleTypeFeature, BundleTypeBugfix, BundleTypePerformance, BundleTypeDocs, BundleTypeChore}
	for _, et := range expectedTypes {
		if typeCount[et] == 0 {
			t.Errorf("expected at least one bundle of type %q", et)
		}
	}
}

func TestBundler_BundleChangesForMonorepo(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("aaa", changes.CommitTypeFeat, "add user service",
		changes.WithScope("api/users"),
	))
	cs.AddCommit(changes.NewConventionalCommit("bbb", changes.CommitTypeFeat, "add order service",
		changes.WithScope("api/orders"),
	))
	cs.AddCommit(changes.NewConventionalCommit("ccc", changes.CommitTypeFix, "fix web layout",
		changes.WithScope("web"),
	))

	componentBundles := b.BundleChangesForMonorepo(cs)
	if componentBundles == nil {
		t.Fatal("expected component bundles")
	}

	// Should group by scope prefix
	if len(componentBundles) == 0 {
		t.Error("expected at least one component group")
	}
}

func TestBundler_BundleChanges_NoScope(t *testing.T) {
	b := NewBundler()

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("aaa", changes.CommitTypeFeat, "add initial feature"))
	cs.AddCommit(changes.NewConventionalCommit("bbb", changes.CommitTypeFeat, "add another feature"))

	bundles := b.BundleChanges(cs)
	if len(bundles) == 0 {
		t.Fatal("expected at least one bundle")
	}

	// Commits without scope should be grouped together
	var featureBundles []Bundle
	for _, bundle := range bundles {
		if bundle.Type == BundleTypeFeature {
			featureBundles = append(featureBundles, bundle)
		}
	}

	if len(featureBundles) != 1 {
		t.Errorf("expected 1 feature bundle for scope-less commits, got %d", len(featureBundles))
	}
}

func TestCollectBundleItems(t *testing.T) {
	bundles := []Bundle{
		{
			Type:    BundleTypeFeature,
			Summary: "New search capability",
			Changes: []BundledChange{
				{Description: "add full-text search"},
			},
		},
		{
			Type: BundleTypeBugfix,
			Changes: []BundledChange{
				{Description: "fix null pointer"},
				{Description: "fix timeout"},
			},
		},
	}

	features := collectBundleItems(bundles, BundleTypeFeature)
	if len(features) != 1 || features[0] != "New search capability" {
		t.Errorf("expected summary as item, got %v", features)
	}

	fixes := collectBundleItems(bundles, BundleTypeBugfix)
	if len(fixes) != 2 {
		t.Errorf("expected 2 bugfix items, got %d", len(fixes))
	}

	breaking := collectBundleItems(bundles, BundleTypeBreaking)
	if len(breaking) != 0 {
		t.Errorf("expected 0 breaking items, got %d", len(breaking))
	}
}

func TestCollectAuthors(t *testing.T) {
	bundles := []Bundle{
		{
			Changes: []BundledChange{
				{Author: "Alice"},
				{Author: "Bob"},
				{Author: "Alice"}, // duplicate
			},
		},
		{
			Changes: []BundledChange{
				{Author: "Carol"},
				{Author: ""}, // empty
			},
		},
	}

	authors := collectAuthors(bundles)
	if len(authors) != 3 {
		t.Errorf("expected 3 unique authors, got %d: %v", len(authors), authors)
	}
}
