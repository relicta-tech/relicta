package release

import (
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

func TestStateSpecification(t *testing.T) {
	rel := NewRelease("test-1", "main", "/repo")

	t.Run("matches initial state", func(t *testing.T) {
		spec := ByState(StateInitialized)
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match initialized release")
		}
	})

	t.Run("does not match different state", func(t *testing.T) {
		spec := ByState(StatePublished)
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match initialized release")
		}
	})
}

func TestActiveSpecification(t *testing.T) {
	t.Run("matches active release", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		spec := Active()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match active release")
		}
	})

	t.Run("does not match final release", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		// Setup release to be published
		cs := changes.NewChangeSet("cs-1", "", "HEAD")
		cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "test"))
		nextVer := version.MustParse("0.1.0")
		plan := NewReleasePlan(version.Initial, nextVer, changes.ReleaseTypeMinor, cs, false)
		rel.SetPlan(plan)
		rel.SetVersion(nextVer, "v0.1.0")
		rel.SetNotes(&ReleaseNotes{Changelog: "test", Summary: "test"})
		rel.Approve("tester", false)
		rel.StartPublishing(nil)
		rel.MarkPublished("http://example.com")

		spec := Active()
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match published release")
		}
	})
}

func TestFinalSpecification(t *testing.T) {
	t.Run("matches final release", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		// Setup release to be published
		cs := changes.NewChangeSet("cs-1", "", "HEAD")
		cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "test"))
		nextVer := version.MustParse("0.1.0")
		plan := NewReleasePlan(version.Initial, nextVer, changes.ReleaseTypeMinor, cs, false)
		rel.SetPlan(plan)
		rel.SetVersion(nextVer, "v0.1.0")
		rel.SetNotes(&ReleaseNotes{Changelog: "test", Summary: "test"})
		rel.Approve("tester", false)
		rel.StartPublishing(nil)
		rel.MarkPublished("http://example.com")

		spec := Final()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match published release")
		}
	})

	t.Run("does not match active release", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		spec := Final()
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match active release")
		}
	})
}

func TestRepositoryPathSpecification(t *testing.T) {
	rel := NewRelease("test-1", "main", "/my/repo")

	t.Run("matches correct path", func(t *testing.T) {
		spec := ByRepositoryPath("/my/repo")
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match repository path")
		}
	})

	t.Run("does not match different path", func(t *testing.T) {
		spec := ByRepositoryPath("/other/repo")
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match different path")
		}
	})
}

func TestBranchSpecification(t *testing.T) {
	rel := NewRelease("test-1", "develop", "/repo")

	t.Run("matches correct branch", func(t *testing.T) {
		spec := ByBranch("develop")
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match branch")
		}
	})

	t.Run("does not match different branch", func(t *testing.T) {
		spec := ByBranch("main")
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match different branch")
		}
	})
}

func TestAndSpecification(t *testing.T) {
	rel := NewRelease("test-1", "main", "/my/repo")

	t.Run("matches when all specs match", func(t *testing.T) {
		spec := And(
			ByBranch("main"),
			ByRepositoryPath("/my/repo"),
			Active(),
		)
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected AND specification to match when all match")
		}
	})

	t.Run("does not match when one spec fails", func(t *testing.T) {
		spec := And(
			ByBranch("main"),
			ByRepositoryPath("/other/repo"),
			Active(),
		)
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected AND specification to not match when one fails")
		}
	})

	t.Run("empty specs matches all", func(t *testing.T) {
		spec := And()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected empty AND specification to match")
		}
	})
}

func TestOrSpecification(t *testing.T) {
	rel := NewRelease("test-1", "main", "/my/repo")

	t.Run("matches when any spec matches", func(t *testing.T) {
		spec := Or(
			ByBranch("develop"),
			ByBranch("main"),
		)
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected OR specification to match when any matches")
		}
	})

	t.Run("does not match when all specs fail", func(t *testing.T) {
		spec := Or(
			ByBranch("develop"),
			ByBranch("feature"),
		)
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected OR specification to not match when all fail")
		}
	})

	t.Run("empty specs matches all", func(t *testing.T) {
		spec := Or()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected empty OR specification to match")
		}
	})
}

func TestNotSpecification(t *testing.T) {
	rel := NewRelease("test-1", "main", "/repo")

	t.Run("negates matching spec", func(t *testing.T) {
		spec := Not(ByBranch("main"))
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected NOT specification to negate match")
		}
	})

	t.Run("negates non-matching spec", func(t *testing.T) {
		spec := Not(ByBranch("develop"))
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected NOT specification to match negated non-match")
		}
	})
}

func TestCompositeSpecifications(t *testing.T) {
	rel := NewRelease("test-1", "main", "/my/repo")

	t.Run("complex composition", func(t *testing.T) {
		// (main branch AND /my/repo) OR (develop branch)
		spec := Or(
			And(ByBranch("main"), ByRepositoryPath("/my/repo")),
			ByBranch("develop"),
		)
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected complex composition to match")
		}
	})

	t.Run("active and not published", func(t *testing.T) {
		spec := And(Active(), Not(ByState(StatePublished)))
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected active and not published to match")
		}
	})
}

func TestHasPlanSpecification(t *testing.T) {
	t.Run("matches release with plan", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		cs := changes.NewChangeSet("cs-1", "", "HEAD")
		cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "test"))
		nextVer := version.MustParse("0.1.0")
		plan := NewReleasePlan(version.Initial, nextVer, changes.ReleaseTypeMinor, cs, false)
		rel.SetPlan(plan)

		spec := HasPlan()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match release with plan")
		}
	})

	t.Run("does not match release without plan", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		spec := HasPlan()
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match release without plan")
		}
	})
}

func TestHasNotesSpecification(t *testing.T) {
	t.Run("matches release with notes", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		cs := changes.NewChangeSet("cs-1", "", "HEAD")
		cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "test"))
		nextVer := version.MustParse("0.1.0")
		plan := NewReleasePlan(version.Initial, nextVer, changes.ReleaseTypeMinor, cs, false)
		rel.SetPlan(plan)
		rel.SetVersion(nextVer, "v0.1.0")
		rel.SetNotes(&ReleaseNotes{Changelog: "test", Summary: "test"})

		spec := HasNotes()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match release with notes")
		}
	})

	t.Run("does not match release without notes", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		spec := HasNotes()
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match release without notes")
		}
	})
}

func TestIsApprovedSpecification(t *testing.T) {
	t.Run("matches approved release", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		cs := changes.NewChangeSet("cs-1", "", "HEAD")
		cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "test"))
		nextVer := version.MustParse("0.1.0")
		plan := NewReleasePlan(version.Initial, nextVer, changes.ReleaseTypeMinor, cs, false)
		rel.SetPlan(plan)
		rel.SetVersion(nextVer, "v0.1.0")
		rel.SetNotes(&ReleaseNotes{Changelog: "test", Summary: "test"})
		rel.Approve("tester", false)

		spec := IsApproved()
		if !spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to match approved release")
		}
	})

	t.Run("does not match unapproved release", func(t *testing.T) {
		rel := NewRelease("test-1", "main", "/repo")
		spec := IsApproved()
		if spec.IsSatisfiedBy(rel) {
			t.Error("expected specification to not match unapproved release")
		}
	})
}
