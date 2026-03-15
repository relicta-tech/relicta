package version

import (
	"testing"
)

func TestLookupChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		channel string
		wantErr bool
	}{
		{"stable", "stable", false},
		{"canary", "canary", false},
		{"alpha", "alpha", false},
		{"beta", "beta", false},
		{"next", "next", false},
		{"unknown", "nightly", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ch, err := LookupChannel(tt.channel)
			if (err != nil) != tt.wantErr {
				t.Errorf("LookupChannel(%q) error = %v, wantErr %v", tt.channel, err, tt.wantErr)
				return
			}
			if !tt.wantErr && ch.Name() != tt.channel {
				t.Errorf("LookupChannel(%q).Name() = %q", tt.channel, ch.Name())
			}
		})
	}
}

func TestChannel_StabilityOrdering(t *testing.T) {
	t.Parallel()

	channels := []struct {
		name      string
		stability StabilityLevel
	}{
		{"canary", StabilityCanary},
		{"alpha", StabilityAlpha},
		{"beta", StabilityBeta},
		{"next", StabilityNext},
		{"stable", StabilityStable},
	}

	for i := 0; i < len(channels)-1; i++ {
		if channels[i].stability >= channels[i+1].stability {
			t.Errorf("expected %s stability (%d) < %s stability (%d)",
				channels[i].name, channels[i].stability,
				channels[i+1].name, channels[i+1].stability)
		}
	}
}

func TestChannel_CanPromoteTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		from  string
		to    string
		canDo bool
	}{
		{"canary to alpha", "canary", "alpha", true},
		{"canary to beta", "canary", "beta", true},
		{"canary to stable", "canary", "stable", true},
		{"alpha to beta", "alpha", "beta", true},
		{"alpha to stable", "alpha", "stable", true},
		{"beta to stable", "beta", "stable", true},
		{"beta to next", "beta", "next", true},
		{"next to stable", "next", "stable", true},
		{"stable to canary", "stable", "canary", false},
		{"beta to alpha", "beta", "alpha", false},
		{"stable to nowhere", "stable", "alpha", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ch, err := LookupChannel(tt.from)
			if err != nil {
				t.Fatalf("LookupChannel(%q) failed: %v", tt.from, err)
			}
			got := ch.CanPromoteTo(tt.to)
			if got != tt.canDo {
				t.Errorf("Channel(%q).CanPromoteTo(%q) = %v, want %v", tt.from, tt.to, got, tt.canDo)
			}
		})
	}
}

func TestChannel_ValidatePromotion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"canary to alpha", "canary", "alpha", false},
		{"alpha to beta", "alpha", "beta", false},
		{"beta to next", "beta", "next", false},
		{"next to stable", "next", "stable", false},
		{"canary to stable", "canary", "stable", false},
		{"stable to canary", "stable", "canary", true},
		{"beta to alpha", "beta", "alpha", true},
		{"stable to beta", "stable", "beta", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			from, _ := LookupChannel(tt.from)
			to, _ := LookupChannel(tt.to)
			err := from.ValidatePromotion(to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePromotion(%q, %q) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestChannel_PrereleaseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		channel    string
		prerelease Prerelease
	}{
		{"stable", ""},
		{"canary", "canary"},
		{"alpha", PrereleaseAlpha},
		{"beta", PrereleaseBeta},
		{"next", PrereleaseRC},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			t.Parallel()
			ch, _ := LookupChannel(tt.channel)
			if ch.PrereleaseID() != tt.prerelease {
				t.Errorf("Channel(%q).PrereleaseID() = %q, want %q", tt.channel, ch.PrereleaseID(), tt.prerelease)
			}
		})
	}
}

func TestChannel_IsStableChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		channel  string
		isStable bool
	}{
		{"stable", true},
		{"canary", false},
		{"alpha", false},
		{"beta", false},
		{"next", false},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			t.Parallel()
			ch, _ := LookupChannel(tt.channel)
			if ch.IsStableChannel() != tt.isStable {
				t.Errorf("Channel(%q).IsStableChannel() = %v, want %v", tt.channel, ch.IsStableChannel(), tt.isStable)
			}
		})
	}
}

func TestChannelRegistry_Get(t *testing.T) {
	t.Parallel()

	reg := NewChannelRegistry()

	ch, err := reg.Get("stable")
	if err != nil {
		t.Fatalf("Get(stable) failed: %v", err)
	}
	if ch.Name() != "stable" {
		t.Errorf("Get(stable).Name() = %q, want stable", ch.Name())
	}

	_, err = reg.Get("unknown")
	if err == nil {
		t.Error("Get(unknown) should return an error")
	}
}

func TestChannelRegistry_Default(t *testing.T) {
	t.Parallel()

	reg := NewChannelRegistry()

	def := reg.Default()
	if def.Name() != "stable" {
		t.Errorf("Default().Name() = %q, want stable", def.Name())
	}
}

func TestChannelRegistry_SetDefault(t *testing.T) {
	t.Parallel()

	reg := NewChannelRegistry()

	err := reg.SetDefault("canary")
	if err != nil {
		t.Fatalf("SetDefault(canary) failed: %v", err)
	}
	if reg.Default().Name() != "canary" {
		t.Errorf("Default().Name() = %q, want canary", reg.Default().Name())
	}

	err = reg.SetDefault("nonexistent")
	if err == nil {
		t.Error("SetDefault(nonexistent) should return an error")
	}
}

func TestChannelRegistry_Register(t *testing.T) {
	t.Parallel()

	reg := NewChannelRegistry()

	custom := NewChannel("nightly", StabilityCanary-1, "v{version}-nightly.{n}", []string{"canary", "alpha"}, "nightly")
	reg.Register(custom)

	ch, err := reg.Get("nightly")
	if err != nil {
		t.Fatalf("Get(nightly) failed: %v", err)
	}
	if ch.Name() != "nightly" {
		t.Errorf("Get(nightly).Name() = %q", ch.Name())
	}
	if ch.PrereleaseID() != "nightly" {
		t.Errorf("Get(nightly).PrereleaseID() = %q", ch.PrereleaseID())
	}
}

func TestChannelRegistry_List(t *testing.T) {
	t.Parallel()

	reg := NewChannelRegistry()
	channels := reg.List()

	if len(channels) != 5 {
		t.Fatalf("List() returned %d channels, want 5", len(channels))
	}

	// Verify sorted by stability
	for i := 0; i < len(channels)-1; i++ {
		if channels[i].Stability() > channels[i+1].Stability() {
			t.Errorf("channels not sorted by stability: %s (%d) > %s (%d)",
				channels[i].Name(), channels[i].Stability(),
				channels[i+1].Name(), channels[i+1].Stability())
		}
	}
}

func TestChannelRegistry_PromotionPath(t *testing.T) {
	t.Parallel()

	reg := NewChannelRegistry()

	tests := []struct {
		name    string
		from    string
		to      string
		wantLen int
		wantErr bool
	}{
		{"canary to stable", "canary", "stable", 2, false},
		{"alpha to stable", "alpha", "stable", 2, false},
		{"canary to alpha", "canary", "alpha", 2, false},
		{"canary to beta", "canary", "beta", 2, false},
		{"stable to canary", "stable", "canary", 0, true},
		{"same channel", "alpha", "alpha", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path, err := reg.PromotionPath(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("PromotionPath(%q, %q) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(path) != tt.wantLen {
				names := make([]string, len(path))
				for i, ch := range path {
					names[i] = ch.Name()
				}
				t.Errorf("PromotionPath(%q, %q) len = %d, want %d, path = %v",
					tt.from, tt.to, len(path), tt.wantLen, names)
			}
		})
	}
}

func TestNewChannel(t *testing.T) {
	t.Parallel()

	ch := NewChannel("staging", StabilityLevel(45), "v{version}-staging.{n}", []string{"stable"}, "staging")

	if ch.Name() != "staging" {
		t.Errorf("Name() = %q, want staging", ch.Name())
	}
	if ch.Stability() != StabilityLevel(45) {
		t.Errorf("Stability() = %d, want 45", ch.Stability())
	}
	if ch.TagPattern() != "v{version}-staging.{n}" {
		t.Errorf("TagPattern() = %q", ch.TagPattern())
	}
	if len(ch.PromotesTo()) != 1 || ch.PromotesTo()[0] != "stable" {
		t.Errorf("PromotesTo() = %v", ch.PromotesTo())
	}
	if ch.PrereleaseID() != "staging" {
		t.Errorf("PrereleaseID() = %q", ch.PrereleaseID())
	}
}

func TestRegisterChannel(t *testing.T) {
	// Reset after test to avoid affecting other tests that use knownChannels
	original := make(map[string]Channel)
	for k, v := range knownChannels {
		original[k] = v
	}
	defer func() {
		knownChannels = original
	}()

	ch := NewChannel("staging", StabilityLevel(45), "v{version}-staging.{n}", []string{"stable"}, "staging")
	err := RegisterChannel(ch)
	if err != nil {
		t.Fatalf("RegisterChannel failed: %v", err)
	}

	got, err := LookupChannel("staging")
	if err != nil {
		t.Fatalf("LookupChannel(staging) failed: %v", err)
	}
	if got.Name() != "staging" {
		t.Errorf("registered channel name = %q, want staging", got.Name())
	}

	// Empty name should fail
	err = RegisterChannel(NewChannel("", StabilityLevel(1), "", nil, ""))
	if err == nil {
		t.Error("RegisterChannel with empty name should fail")
	}
}
