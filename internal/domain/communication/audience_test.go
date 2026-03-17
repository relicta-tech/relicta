package communication

import (
	"testing"
)

func TestIsValidAudienceType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "engineering valid", input: "engineering", want: true},
		{name: "product valid", input: "product", want: true},
		{name: "executive valid", input: "executive", want: true},
		{name: "external valid", input: "external", want: true},
		{name: "empty invalid", input: "", want: false},
		{name: "unknown invalid", input: "marketing", want: false},
		{name: "uppercase invalid", input: "Engineering", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidAudienceType(tt.input)
			if got != tt.want {
				t.Errorf("IsValidAudienceType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllAudienceTypes(t *testing.T) {
	types := AllAudienceTypes()
	if len(types) != 4 {
		t.Errorf("AllAudienceTypes() returned %d types, want 4", len(types))
	}

	expected := map[AudienceType]bool{
		AudienceEngineering: true,
		AudienceProduct:     true,
		AudienceExecutive:   true,
		AudienceExternal:    true,
	}

	for _, at := range types {
		if !expected[at] {
			t.Errorf("unexpected audience type %q", at)
		}
	}
}

func TestAudienceValidate(t *testing.T) {
	tests := []struct {
		name    string
		aud     Audience
		wantErr bool
	}{
		{
			name: "valid engineering audience",
			aud: Audience{
				Type:        AudienceEngineering,
				Name:        "Engineering",
				Tone:        CommToneTechnical,
				DetailLevel: DetailFull,
				Sections:    []Section{SectionFeatures},
			},
			wantErr: false,
		},
		{
			name: "missing type",
			aud: Audience{
				Tone:        CommToneTechnical,
				DetailLevel: DetailFull,
				Sections:    []Section{SectionFeatures},
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			aud: Audience{
				Type:        "invalid",
				Tone:        CommToneTechnical,
				DetailLevel: DetailFull,
				Sections:    []Section{SectionFeatures},
			},
			wantErr: true,
		},
		{
			name: "missing tone",
			aud: Audience{
				Type:        AudienceEngineering,
				DetailLevel: DetailFull,
				Sections:    []Section{SectionFeatures},
			},
			wantErr: true,
		},
		{
			name: "missing detail level",
			aud: Audience{
				Type:     AudienceEngineering,
				Tone:     CommToneTechnical,
				Sections: []Section{SectionFeatures},
			},
			wantErr: true,
		},
		{
			name: "missing sections",
			aud: Audience{
				Type:        AudienceEngineering,
				Tone:        CommToneTechnical,
				DetailLevel: DetailFull,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.aud.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAudiences(t *testing.T) {
	defaults := DefaultAudiences()

	if len(defaults) != 4 {
		t.Errorf("DefaultAudiences() returned %d, want 4", len(defaults))
	}

	// Verify each default audience is valid
	for at, aud := range defaults {
		if err := aud.Validate(); err != nil {
			t.Errorf("default audience %q is invalid: %v", at, err)
		}
	}

	// Engineering should be technical with full detail
	eng := defaults[AudienceEngineering]
	if eng.Tone != CommToneTechnical {
		t.Errorf("engineering tone = %q, want %q", eng.Tone, CommToneTechnical)
	}
	if eng.DetailLevel != DetailFull {
		t.Errorf("engineering detail = %q, want %q", eng.DetailLevel, DetailFull)
	}

	// Executive should be executive tone with highlights
	exec := defaults[AudienceExecutive]
	if exec.Tone != CommToneExecutive {
		t.Errorf("executive tone = %q, want %q", exec.Tone, CommToneExecutive)
	}
	if exec.DetailLevel != DetailHighlights {
		t.Errorf("executive detail = %q, want %q", exec.DetailLevel, DetailHighlights)
	}
}

func TestCommunicationConfigResolveAudience(t *testing.T) {
	t.Run("resolves default audience", func(t *testing.T) {
		cfg := DefaultCommunicationConfig()
		aud, err := cfg.ResolveAudience(AudienceEngineering)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aud.Type != AudienceEngineering {
			t.Errorf("type = %q, want %q", aud.Type, AudienceEngineering)
		}
	})

	t.Run("resolves custom audience", func(t *testing.T) {
		cfg := CommunicationConfig{
			DefaultAudience: AudienceEngineering,
			Audiences: map[AudienceType]AudienceConfig{
				AudienceEngineering: {
					Name:        "Dev Team",
					Tone:        "technical",
					DetailLevel: "full",
					Sections:    []string{"features", "fixes"},
				},
			},
		}
		aud, err := cfg.ResolveAudience(AudienceEngineering)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aud.Name != "Dev Team" {
			t.Errorf("name = %q, want %q", aud.Name, "Dev Team")
		}
	})

	t.Run("error on unknown audience", func(t *testing.T) {
		cfg := DefaultCommunicationConfig()
		_, err := cfg.ResolveAudience("unknown")
		if err == nil {
			t.Error("expected error for unknown audience type")
		}
	})
}

func TestCommunicationConfigResolveAllAudiences(t *testing.T) {
	cfg := DefaultCommunicationConfig()
	audiences, err := cfg.ResolveAllAudiences()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audiences) != 4 {
		t.Errorf("got %d audiences, want 4", len(audiences))
	}
}

func TestIsValidOutputFormat(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"markdown", true},
		{"plaintext", true},
		{"html", true},
		{"pdf", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidOutputFormat(tt.input)
			if got != tt.want {
				t.Errorf("IsValidOutputFormat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
