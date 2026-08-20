package container

import (
	"context"
	"strings"
	"testing"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// `publish --skip-tag` reached the printed summary and the JSON output and nothing else.
// Verified against the shipped binary before this gate existed:
//
//	$ relicta publish --skip-tag
//	  Create tag: false
//	  ...
//	$ git tag
//	v0.1.0
//
// versioning.git_tag had no reader either, so a repository that set it to false was tagged
// on every release. The tag step is put into the execution plan at approve time, which is
// why the flag had nothing left to act on by publish: the decision has to be made where the
// step runs.
//
// This is the same defect as the push one next door, one flag along.

func TestPublisher_TaggingDisabledCreatesNoTag(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags, WithTagging(false), WithPushTags(true))

	result, err := publisher.executeTagStep(context.Background(), tagStepRun(t))
	if err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.created) != 0 {
		t.Errorf("tag %v was created with tagging disabled", tags.created)
	}
	if len(tags.pushed) != 0 {
		t.Errorf("tag %v was pushed with tagging disabled; a tag that was never created "+
			"cannot be pushed, so this would be a second defect on top of the first", tags.pushed)
	}
	if !result.Success {
		t.Error("the step reported failure. Declining to tag is the configured outcome, " +
			"not an error, and failing here would abort a release the operator asked for")
	}
	if !strings.Contains(result.Output, "not created") {
		t.Errorf("the step result reads %q, which does not say the tag was not created. "+
			"The run's record is what an auditor reads later", result.Output)
	}
}

func TestPublisher_TaggingEnabledCreatesTheTag(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags, WithTagging(true), WithPushTags(false))

	if _, err := publisher.executeTagStep(context.Background(), tagStepRun(t)); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.created) != 1 {
		t.Errorf("expected one tag to be created, got %v", tags.created)
	}
}

// A publisher built without options must leave the repository's tags alone, so that a
// construction site which forgets the option gets the harmless behavior. Every construction
// site did forget the push option, which is how that defect survived.
func TestPublisher_DefaultCreatesNoTag(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags)

	if _, err := publisher.executeTagStep(context.Background(), tagStepRun(t)); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.created) != 0 {
		t.Errorf("a publisher constructed without options created %v", tags.created)
	}
}

// A monorepo release carries one tag per package, alongside the repository's own.
//
// Alongside, not instead: the repository tag stays the marker the repository-wide commands
// measure from, and a monorepo with none would count from the start of history forever.

func packageTagsFor(tags ...appmonorepo.PackageTag) func(context.Context) ([]appmonorepo.PackageTag, error) {
	return func(context.Context) ([]appmonorepo.PackageTag, error) { return tags, nil }
}

func TestPublisher_TagsEachPackageAlongsideTheRepository(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags,
		WithTagging(true),
		WithPackageTags(packageTagsFor(
			appmonorepo.PackageTag{Name: "api", RelPath: "packages/api", Tag: "api-v1.5.0", Version: version.MustParse("1.5.0")},
			appmonorepo.PackageTag{Name: "web", RelPath: "packages/web", Tag: "web-v2.1.4", Version: version.MustParse("2.1.4")},
		)))

	result, err := publisher.executeTagStep(context.Background(), tagStepRun(t))
	if err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	// v0.0.0 is what the shared fixture's run carries; the point here is that it is created
	// as well as the package tags, not instead of them.
	want := map[string]bool{"v0.0.0": true, "api-v1.5.0": true, "web-v2.1.4": true}
	if len(tags.created) != len(want) {
		t.Fatalf("created %v, want the repository tag and one per package", tags.created)
	}
	for _, created := range tags.created {
		if !want[created] {
			t.Errorf("unexpected tag %q", created)
		}
	}
	if !strings.Contains(result.Output, "api-v1.5.0") {
		t.Errorf("the step result %q does not name the package tags it created", result.Output)
	}
}

// Re-running publish after a partial failure must finish the job, not refuse it.
func TestPublisher_PackageTagsAreIdempotent(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{"api-v1.5.0": true}}
	publisher := NewPublisherAdapter(nil, nil, tags,
		WithTagging(true),
		WithPackageTags(packageTagsFor(
			appmonorepo.PackageTag{Name: "api", RelPath: "packages/api", Tag: "api-v1.5.0", Version: version.MustParse("1.5.0")},
			appmonorepo.PackageTag{Name: "web", RelPath: "packages/web", Tag: "web-v2.1.4", Version: version.MustParse("2.1.4")},
		)))

	if _, err := publisher.executeTagStep(context.Background(), tagStepRun(t)); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	for _, created := range tags.created {
		if created == "api-v1.5.0" {
			t.Error("a package tag that already existed was created again")
		}
	}
	var taggedWeb bool
	for _, created := range tags.created {
		if created == "web-v2.1.4" {
			taggedWeb = true
		}
	}
	if !taggedWeb {
		t.Errorf("web was not tagged: %v. One existing tag stopped the rest of the release",
			tags.created)
	}
}

// Tagging disabled means no tags at all, package ones included.
func TestPublisher_TaggingDisabledSkipsPackageTagsToo(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags,
		WithTagging(false),
		WithPackageTags(packageTagsFor(
			appmonorepo.PackageTag{Name: "api", RelPath: "packages/api", Tag: "api-v1.5.0", Version: version.MustParse("1.5.0")},
		)))

	if _, err := publisher.executeTagStep(context.Background(), tagStepRun(t)); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.created) != 0 {
		t.Errorf("created %v with tagging disabled; --skip-tag has to mean every tag", tags.created)
	}
}
