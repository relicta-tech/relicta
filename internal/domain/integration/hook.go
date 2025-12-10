// Package integration provides domain types for plugin integration.
package integration

// Hook represents a plugin hook point in the release workflow.
type Hook string

const (
	// HookPreInit is called before initialization.
	HookPreInit Hook = "pre-init"
	// HookPostInit is called after initialization.
	HookPostInit Hook = "post-init"

	// HookPrePlan is called before planning.
	HookPrePlan Hook = "pre-plan"
	// HookPostPlan is called after planning.
	HookPostPlan Hook = "post-plan"

	// HookPreVersion is called before versioning.
	HookPreVersion Hook = "pre-version"
	// HookPostVersion is called after versioning.
	HookPostVersion Hook = "post-version"

	// HookPreNotes is called before generating notes.
	HookPreNotes Hook = "pre-notes"
	// HookPostNotes is called after generating notes.
	HookPostNotes Hook = "post-notes"

	// HookPreApprove is called before approval.
	HookPreApprove Hook = "pre-approve"
	// HookPostApprove is called after approval.
	HookPostApprove Hook = "post-approve"

	// HookPrePublish is called before publishing.
	HookPrePublish Hook = "pre-publish"
	// HookPostPublish is called after publishing.
	HookPostPublish Hook = "post-publish"

	// HookOnSuccess is called when release succeeds.
	HookOnSuccess Hook = "on-success"
	// HookOnError is called when release fails.
	HookOnError Hook = "on-error"
)

// String returns the string representation of the hook.
func (h Hook) String() string {
	return string(h)
}

// IsValid returns true if the hook is a valid hook.
func (h Hook) IsValid() bool {
	switch h {
	case HookPreInit, HookPostInit,
		HookPrePlan, HookPostPlan,
		HookPreVersion, HookPostVersion,
		HookPreNotes, HookPostNotes,
		HookPreApprove, HookPostApprove,
		HookPrePublish, HookPostPublish,
		HookOnSuccess, HookOnError:
		return true
	default:
		return false
	}
}

// IsPre returns true if this is a pre-hook.
func (h Hook) IsPre() bool {
	switch h {
	case HookPreInit, HookPrePlan, HookPreVersion,
		HookPreNotes, HookPreApprove, HookPrePublish:
		return true
	default:
		return false
	}
}

// IsPost returns true if this is a post-hook.
func (h Hook) IsPost() bool {
	switch h {
	case HookPostInit, HookPostPlan, HookPostVersion,
		HookPostNotes, HookPostApprove, HookPostPublish:
		return true
	default:
		return false
	}
}

// IsLifecycle returns true if this is a lifecycle hook (success/error).
func (h Hook) IsLifecycle() bool {
	return h == HookOnSuccess || h == HookOnError
}

// AllHooks returns all valid hooks in execution order.
func AllHooks() []Hook {
	return []Hook{
		HookPreInit, HookPostInit,
		HookPrePlan, HookPostPlan,
		HookPreVersion, HookPostVersion,
		HookPreNotes, HookPostNotes,
		HookPreApprove, HookPostApprove,
		HookPrePublish, HookPostPublish,
		HookOnSuccess, HookOnError,
	}
}

// PreHooks returns all pre-hooks in order.
func PreHooks() []Hook {
	return []Hook{
		HookPreInit, HookPrePlan, HookPreVersion,
		HookPreNotes, HookPreApprove, HookPrePublish,
	}
}

// PostHooks returns all post-hooks in order.
func PostHooks() []Hook {
	return []Hook{
		HookPostInit, HookPostPlan, HookPostVersion,
		HookPostNotes, HookPostApprove, HookPostPublish,
	}
}

// HookPair returns the corresponding pre/post hook pair.
func (h Hook) HookPair() (Hook, Hook) {
	switch h {
	case HookPreInit, HookPostInit:
		return HookPreInit, HookPostInit
	case HookPrePlan, HookPostPlan:
		return HookPrePlan, HookPostPlan
	case HookPreVersion, HookPostVersion:
		return HookPreVersion, HookPostVersion
	case HookPreNotes, HookPostNotes:
		return HookPreNotes, HookPostNotes
	case HookPreApprove, HookPostApprove:
		return HookPreApprove, HookPostApprove
	case HookPrePublish, HookPostPublish:
		return HookPrePublish, HookPostPublish
	default:
		return h, h
	}
}

// Description returns a human-readable description of the hook.
func (h Hook) Description() string {
	switch h {
	case HookPreInit:
		return "Before release initialization"
	case HookPostInit:
		return "After release initialization"
	case HookPrePlan:
		return "Before release planning"
	case HookPostPlan:
		return "After release planning"
	case HookPreVersion:
		return "Before version calculation"
	case HookPostVersion:
		return "After version calculation"
	case HookPreNotes:
		return "Before release notes generation"
	case HookPostNotes:
		return "After release notes generation"
	case HookPreApprove:
		return "Before release approval"
	case HookPostApprove:
		return "After release approval"
	case HookPrePublish:
		return "Before publishing release"
	case HookPostPublish:
		return "After publishing release"
	case HookOnSuccess:
		return "On successful release"
	case HookOnError:
		return "On release error"
	default:
		return "Unknown hook"
	}
}
