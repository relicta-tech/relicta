package container

import (
	"context"
	"strings"
	"testing"
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
