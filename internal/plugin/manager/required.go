// Package manager provides plugin management functionality for Relicta.
package manager

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Requirement is one entry of plugins_security.required: a plugin name with
// an optional semver constraint ("github", "github@^2.0", "slack@>=1.2").
type Requirement struct {
	// Name of the plugin.
	Name string
	// Constraint is the parsed semver constraint; nil means "any version".
	Constraint *semver.Constraints
	// Raw is the original spec string, kept for error messages.
	Raw string
}

// ParseRequirement parses a "name[@constraint]" spec.
func ParseRequirement(spec string) (Requirement, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		return Requirement{}, fmt.Errorf("empty plugin requirement")
	}

	name, constraintStr, found := strings.Cut(raw, "@")
	name = strings.TrimSpace(name)
	if name == "" {
		return Requirement{}, fmt.Errorf("requirement %q: missing plugin name", raw)
	}

	req := Requirement{Name: name, Raw: raw}
	if found {
		c, err := semver.NewConstraint(strings.TrimSpace(constraintStr))
		if err != nil {
			return Requirement{}, fmt.Errorf("requirement %q: invalid version constraint: %w", raw, err)
		}
		req.Constraint = c
	}
	return req, nil
}

// ParseRequirements parses all specs, collecting every error so the operator
// sees the full list at once.
func ParseRequirements(specs []string) ([]Requirement, error) {
	var (
		reqs []Requirement
		errs []string
	)
	for _, s := range specs {
		r, err := ParseRequirement(s)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		reqs = append(reqs, r)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid plugin requirements:\n  %s", strings.Join(errs, "\n  "))
	}
	return reqs, nil
}

// Satisfies reports whether the given installed version meets the requirement.
// Versions that do not parse as semver never satisfy a constraint.
func (r Requirement) Satisfies(version string) bool {
	if r.Constraint == nil {
		return version != ""
	}
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return false
	}
	return r.Constraint.Check(v)
}

// RequiredAction describes what to do for one requirement.
type RequiredAction struct {
	Requirement Requirement
	// Install is true when the plugin must be (re)installed.
	Install bool
	// Reason explains the action ("not installed", "installed 1.0.0 does
	// not satisfy ^2.0", "satisfied").
	Reason string
}

// ResolveRequired computes the install actions for the required set, given
// the currently installed plugins. Registry availability and version
// matching are checked by the installer at execution time; resolution here
// only decides whether the installed state already satisfies each spec.
func ResolveRequired(reqs []Requirement, installed []InstalledPlugin) []RequiredAction {
	byName := make(map[string]InstalledPlugin, len(installed))
	for _, p := range installed {
		byName[p.Name] = p
	}

	actions := make([]RequiredAction, 0, len(reqs))
	for _, r := range reqs {
		cur, ok := byName[r.Name]
		switch {
		case !ok:
			actions = append(actions, RequiredAction{Requirement: r, Install: true, Reason: "not installed"})
		case !r.Satisfies(cur.Version):
			actions = append(actions, RequiredAction{
				Requirement: r,
				Install:     true,
				Reason:      fmt.Sprintf("installed %s does not satisfy %s", cur.Version, r.Raw),
			})
		default:
			actions = append(actions, RequiredAction{Requirement: r, Install: false, Reason: "satisfied"})
		}
	}
	return actions
}
