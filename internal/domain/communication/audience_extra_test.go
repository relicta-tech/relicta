package communication

import (
	"testing"
)

func TestResolveAllAudiences(t *testing.T) {
	t.Run("default config resolves all audiences", func(t *testing.T) {
		cfg := DefaultCommunicationConfig()
		audiences, err := cfg.ResolveAllAudiences()
		if err != nil {
			t.Fatalf("ResolveAllAudiences() error = %v", err)
		}
		if len(audiences) == 0 {
			t.Error("expected at least 1 audience")
		}
	})

	t.Run("custom audiences override defaults", func(t *testing.T) {
		cfg := DefaultCommunicationConfig()
		cfg.Audiences = make(map[AudienceType]AudienceConfig)
		cfg.Audiences[AudienceEngineering] = AudienceConfig{
			Name:        "Custom Engineering",
			Tone:        "casual",
			DetailLevel: "brief",
			Sections:    []string{"summary", "changes"},
		}

		audiences, err := cfg.ResolveAllAudiences()
		if err != nil {
			t.Fatalf("ResolveAllAudiences() error = %v", err)
		}
		if len(audiences) == 0 {
			t.Error("expected audiences")
		}

		found := false
		for _, a := range audiences {
			if a.Type == AudienceEngineering && a.Name == "Custom Engineering" {
				found = true
			}
		}
		if !found {
			t.Error("expected custom engineering audience")
		}
	})

	t.Run("empty config still resolves defaults", func(t *testing.T) {
		cfg := CommunicationConfig{
			Audiences: make(map[AudienceType]AudienceConfig),
		}

		audiences, err := cfg.ResolveAllAudiences()
		if err != nil {
			t.Fatalf("ResolveAllAudiences() error = %v", err)
		}
		if len(audiences) == 0 {
			t.Error("expected default audiences even with empty config")
		}
	})
}
