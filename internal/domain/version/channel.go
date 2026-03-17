// Package version provides domain types for semantic versioning.
package version

import (
	"fmt"
	"sort"
)

// Channel represents a release channel that governs how versions are published
// and promoted through the release pipeline.
type Channel struct {
	// name is the unique identifier for this channel.
	name string
	// stability is the stability level (higher = more stable).
	stability StabilityLevel
	// tagPattern is the tag pattern for this channel (e.g., "v{version}-canary.{n}").
	tagPattern string
	// promotesTo lists channel names this channel can promote to.
	promotesTo []string
	// prerelease is the prerelease identifier used for versions on this channel.
	// Empty string means stable releases (no prerelease suffix).
	prerelease Prerelease
}

// StabilityLevel represents the stability ranking of a channel.
// Higher values indicate greater stability.
type StabilityLevel int

const (
	// StabilityCanary is the lowest stability level for bleeding-edge releases.
	StabilityCanary StabilityLevel = 10
	// StabilityAlpha is for early development releases.
	StabilityAlpha StabilityLevel = 20
	// StabilityBeta is for feature-complete but potentially unstable releases.
	StabilityBeta StabilityLevel = 30
	// StabilityNext is for release candidates being tested before stable.
	StabilityNext StabilityLevel = 40
	// StabilityRC is for release candidates.
	StabilityRC StabilityLevel = 50
	// StabilityStable is the highest stability level for production releases.
	StabilityStable StabilityLevel = 100
)

// Predefined channels with their default configurations.
var (
	// ChannelStable is the default stable release channel.
	ChannelStable = Channel{
		name:       "stable",
		stability:  StabilityStable,
		tagPattern: "v{version}",
		promotesTo: nil, // Terminal channel
		prerelease: "",
	}

	// ChannelCanary is the canary/nightly release channel.
	ChannelCanary = Channel{
		name:       "canary",
		stability:  StabilityCanary,
		tagPattern: "v{version}-canary.{n}",
		promotesTo: []string{"alpha", "beta", "next", "stable"},
		prerelease: "canary",
	}

	// ChannelAlpha is the alpha release channel.
	ChannelAlpha = Channel{
		name:       "alpha",
		stability:  StabilityAlpha,
		tagPattern: "v{version}-alpha.{n}",
		promotesTo: []string{"beta", "next", "stable"},
		prerelease: PrereleaseAlpha,
	}

	// ChannelBeta is the beta release channel.
	ChannelBeta = Channel{
		name:       "beta",
		stability:  StabilityBeta,
		tagPattern: "v{version}-beta.{n}",
		promotesTo: []string{"next", "stable"},
		prerelease: PrereleaseBeta,
	}

	// ChannelNext is the next/preview release channel.
	ChannelNext = Channel{
		name:       "next",
		stability:  StabilityNext,
		tagPattern: "v{version}-rc.{n}",
		promotesTo: []string{"stable"},
		prerelease: PrereleaseRC,
	}
)

// knownChannels maps channel names to their definitions.
var knownChannels = map[string]Channel{
	"stable": ChannelStable,
	"canary": ChannelCanary,
	"alpha":  ChannelAlpha,
	"beta":   ChannelBeta,
	"next":   ChannelNext,
}

// LookupChannel returns the channel definition for a given name.
// Returns an error if the channel name is not recognized.
func LookupChannel(name string) (Channel, error) {
	ch, ok := knownChannels[name]
	if !ok {
		return Channel{}, fmt.Errorf("unknown channel: %q", name)
	}
	return ch, nil
}

// RegisterChannel registers a custom channel definition.
// This allows extending the predefined set with project-specific channels.
func RegisterChannel(ch Channel) error {
	if ch.name == "" {
		return fmt.Errorf("channel name must not be empty")
	}
	knownChannels[ch.name] = ch
	return nil
}

// NewChannel creates a new Channel value object.
func NewChannel(name string, stability StabilityLevel, tagPattern string, promotesTo []string, prerelease Prerelease) Channel {
	return Channel{
		name:       name,
		stability:  stability,
		tagPattern: tagPattern,
		promotesTo: promotesTo,
		prerelease: prerelease,
	}
}

// Name returns the channel name.
func (c Channel) Name() string {
	return c.name
}

// Stability returns the stability level.
func (c Channel) Stability() StabilityLevel {
	return c.stability
}

// TagPattern returns the tag pattern for this channel.
func (c Channel) TagPattern() string {
	return c.tagPattern
}

// PromotesTo returns the list of channels this channel can promote to.
func (c Channel) PromotesTo() []string {
	return c.promotesTo
}

// PrereleaseID returns the prerelease identifier for this channel.
func (c Channel) PrereleaseID() Prerelease {
	return c.prerelease
}

// IsStableChannel returns true if this is the stable (production) channel.
func (c Channel) IsStableChannel() bool {
	return c.stability == StabilityStable
}

// CanPromoteTo returns true if this channel can promote to the target channel.
func (c Channel) CanPromoteTo(target string) bool {
	for _, t := range c.promotesTo {
		if t == target {
			return true
		}
	}
	return false
}

// ValidatePromotion checks whether promoting from this channel to the target is valid.
// Promotion is only valid to channels with higher stability.
func (c Channel) ValidatePromotion(target Channel) error {
	if !c.CanPromoteTo(target.name) {
		return fmt.Errorf("channel %q cannot promote to %q", c.name, target.name)
	}
	if target.stability <= c.stability {
		return fmt.Errorf("cannot promote to channel %q with equal or lower stability", target.name)
	}
	return nil
}

// ChannelRegistry holds the channel configuration for a project.
type ChannelRegistry struct {
	channels       map[string]Channel
	defaultChannel string
}

// NewChannelRegistry creates a new ChannelRegistry with default channels.
func NewChannelRegistry() *ChannelRegistry {
	return &ChannelRegistry{
		channels: map[string]Channel{
			"stable": ChannelStable,
			"canary": ChannelCanary,
			"alpha":  ChannelAlpha,
			"beta":   ChannelBeta,
			"next":   ChannelNext,
		},
		defaultChannel: "stable",
	}
}

// Get returns a channel by name.
func (r *ChannelRegistry) Get(name string) (Channel, error) {
	ch, ok := r.channels[name]
	if !ok {
		return Channel{}, fmt.Errorf("unknown channel: %q", name)
	}
	return ch, nil
}

// Default returns the default channel.
func (r *ChannelRegistry) Default() Channel {
	ch, ok := r.channels[r.defaultChannel]
	if !ok {
		return ChannelStable
	}
	return ch
}

// SetDefault sets the default channel name.
func (r *ChannelRegistry) SetDefault(name string) error {
	if _, ok := r.channels[name]; !ok {
		return fmt.Errorf("cannot set default to unknown channel: %q", name)
	}
	r.defaultChannel = name
	return nil
}

// Register adds or updates a channel in the registry.
func (r *ChannelRegistry) Register(ch Channel) {
	r.channels[ch.name] = ch
}

// List returns all registered channels sorted by stability level.
func (r *ChannelRegistry) List() []Channel {
	channels := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		channels = append(channels, ch)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].stability < channels[j].stability
	})
	return channels
}

// PromotionPath returns the ordered list of channels from source to target.
// Returns an error if no valid promotion path exists.
func (r *ChannelRegistry) PromotionPath(from, to string) ([]Channel, error) {
	source, err := r.Get(from)
	if err != nil {
		return nil, fmt.Errorf("source channel: %w", err)
	}
	target, err := r.Get(to)
	if err != nil {
		return nil, fmt.Errorf("target channel: %w", err)
	}

	if source.stability >= target.stability {
		return nil, fmt.Errorf("source channel %q has equal or higher stability than target %q", from, to)
	}

	// BFS to find a valid path
	type node struct {
		channel Channel
		path    []Channel
	}

	visited := map[string]bool{from: true}
	queue := []node{{channel: source, path: []Channel{source}}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, nextName := range current.channel.promotesTo {
			if nextName == to {
				return append(current.path, target), nil
			}
			if visited[nextName] {
				continue
			}
			visited[nextName] = true
			next, err := r.Get(nextName)
			if err != nil {
				continue
			}
			queue = append(queue, node{
				channel: next,
				path:    append(append([]Channel{}, current.path...), next),
			})
		}
	}

	return nil, fmt.Errorf("no promotion path from %q to %q", from, to)
}
