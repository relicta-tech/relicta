package release

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/communication"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

func TestValidateReleaseID(t *testing.T) {
	tests := []struct {
		name    string
		id      release.ReleaseID
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid release ID",
			id:      "rel-1234567890",
			wantErr: false,
		},
		{
			name:    "empty release ID",
			id:      "",
			wantErr: true,
			errMsg:  "release ID is required",
		},
		{
			name:    "valid alphanumeric",
			id:      "release123",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			id:      "test-release-123",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			id:      "test_release_123",
			wantErr: false,
		},
		{
			name:    "invalid - starts with hyphen",
			id:      "-invalid",
			wantErr: true,
			errMsg:  "invalid release ID format",
		},
		{
			name:    "invalid - contains spaces",
			id:      "invalid release",
			wantErr: true,
			errMsg:  "invalid release ID format",
		},
		{
			name:    "invalid - special characters",
			id:      "release@123",
			wantErr: true,
			errMsg:  "invalid release ID format",
		},
		{
			name:    "too long",
			id:      release.ReleaseID("rel-" + strings.Repeat("1", 100)),
			wantErr: true,
			errMsg:  "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReleaseID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReleaseID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateReleaseID() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateSafeString(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		fieldName string
		maxLen    int
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid string",
			s:         "valid-string",
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   false,
		},
		{
			name:      "empty string is valid",
			s:         "",
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   false,
		},
		{
			name:      "too long",
			s:         strings.Repeat("a", 300),
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   true,
			errMsg:    "too long",
		},
		{
			name:      "contains newline",
			s:         "has\nnewline",
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   true,
			errMsg:    "invalid control characters",
		},
		{
			name:      "contains carriage return",
			s:         "has\rreturn",
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   true,
			errMsg:    "invalid control characters",
		},
		{
			name:      "contains tab",
			s:         "has\ttab",
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   true,
			errMsg:    "invalid control characters",
		},
		{
			name:      "contains null byte",
			s:         "has\x00null",
			fieldName: "test_field",
			maxLen:    256,
			wantErr:   true,
			errMsg:    "invalid control characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSafeString(tt.s, tt.fieldName, tt.maxLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSafeString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSafeString() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name      string
		urlStr    string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid https URL",
			urlStr:    "https://github.com/org/repo",
			fieldName: "repository URL",
			wantErr:   false,
		},
		{
			name:      "valid http URL",
			urlStr:    "http://example.com",
			fieldName: "repository URL",
			wantErr:   false,
		},
		{
			name:      "empty URL is valid",
			urlStr:    "",
			fieldName: "repository URL",
			wantErr:   false,
		},
		{
			name:      "relative path is valid",
			urlStr:    "/path/to/repo",
			fieldName: "repository URL",
			wantErr:   false,
		},
		{
			name:      "too long",
			urlStr:    "https://example.com/" + strings.Repeat("a", 3000),
			fieldName: "repository URL",
			wantErr:   true,
			errMsg:    "too long",
		},
		{
			name:      "invalid scheme",
			urlStr:    "ftp://example.com",
			fieldName: "repository URL",
			wantErr:   true,
			errMsg:    "must be http or https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.urlStr, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateURL() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestApproveReleaseInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ApproveReleaseInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: ApproveReleaseInput{
				ReleaseID:   "rel-1234567890",
				ApprovedBy:  "user@example.com",
				AutoApprove: true,
			},
			wantErr: false,
		},
		{
			name: "empty release ID",
			input: ApproveReleaseInput{
				ReleaseID:  "",
				ApprovedBy: "user@example.com",
			},
			wantErr: true,
			errMsg:  "release ID is required",
		},
		{
			name: "approver too long",
			input: ApproveReleaseInput{
				ReleaseID:  "rel-1234567890",
				ApprovedBy: strings.Repeat("a", 300),
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "approver with newline",
			input: ApproveReleaseInput{
				ReleaseID:  "rel-1234567890",
				ApprovedBy: "user\n@example.com",
			},
			wantErr: true,
			errMsg:  "invalid control characters",
		},
		{
			name: "edited notes too long",
			input: ApproveReleaseInput{
				ReleaseID:   "rel-1234567890",
				EditedNotes: stringPtr(strings.Repeat("a", MaxNotesLength+1)),
			},
			wantErr: true,
			errMsg:  "edited notes too long",
		},
		{
			name: "valid with edited notes",
			input: ApproveReleaseInput{
				ReleaseID:   "rel-1234567890",
				EditedNotes: stringPtr("Updated release notes"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ApproveReleaseInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ApproveReleaseInput.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGenerateNotesInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   GenerateNotesInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: GenerateNotesInput{
				ReleaseID:     "rel-1234567890",
				UseAI:         true,
				Tone:          communication.ToneTechnical,
				Audience:      communication.AudienceDevelopers,
				RepositoryURL: "https://github.com/org/repo",
			},
			wantErr: false,
		},
		{
			name: "valid with empty optional fields",
			input: GenerateNotesInput{
				ReleaseID: "rel-1234567890",
			},
			wantErr: false,
		},
		{
			name: "empty release ID",
			input: GenerateNotesInput{
				ReleaseID: "",
			},
			wantErr: true,
			errMsg:  "release ID is required",
		},
		{
			name: "invalid tone",
			input: GenerateNotesInput{
				ReleaseID: "rel-1234567890",
				Tone:      "invalid-tone",
			},
			wantErr: true,
			errMsg:  "invalid tone",
		},
		{
			name: "invalid audience",
			input: GenerateNotesInput{
				ReleaseID: "rel-1234567890",
				Audience:  "invalid-audience",
			},
			wantErr: true,
			errMsg:  "invalid audience",
		},
		{
			name: "invalid URL scheme",
			input: GenerateNotesInput{
				ReleaseID:     "rel-1234567890",
				RepositoryURL: "ftp://invalid.url",
			},
			wantErr: true,
			errMsg:  "must be http or https",
		},
		{
			name: "URL too long",
			input: GenerateNotesInput{
				ReleaseID:     "rel-1234567890",
				RepositoryURL: "https://example.com/" + strings.Repeat("a", 3000),
			},
			wantErr: true,
			errMsg:  "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateNotesInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("GenerateNotesInput.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGetReleaseForApprovalInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   GetReleaseForApprovalInput
		wantErr bool
	}{
		{
			name:    "valid",
			input:   GetReleaseForApprovalInput{ReleaseID: "rel-1234567890"},
			wantErr: false,
		},
		{
			name:    "empty release ID",
			input:   GetReleaseForApprovalInput{ReleaseID: ""},
			wantErr: true,
		},
		{
			name:    "invalid format - special chars",
			input:   GetReleaseForApprovalInput{ReleaseID: "invalid@id"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleaseForApprovalInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		v := NewValidationError()
		if v.HasErrors() {
			t.Error("HasErrors() should be false for new ValidationError")
		}
		if v.ToError() != nil {
			t.Error("ToError() should return nil for empty ValidationError")
		}
	})

	t.Run("single error", func(t *testing.T) {
		v := NewValidationError()
		v.AddMessage("test error")
		if !v.HasErrors() {
			t.Error("HasErrors() should be true")
		}
		err := v.ToError()
		if err == nil {
			t.Fatal("ToError() should not return nil")
		}
		if err.Error() != "test error" {
			t.Errorf("Error() = %q, want %q", err.Error(), "test error")
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		v := NewValidationError()
		v.AddMessage("error 1")
		v.AddMessage("error 2")
		err := v.ToError()
		if err == nil {
			t.Fatal("ToError() should not return nil")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "error 1") || !strings.Contains(errStr, "error 2") {
			t.Errorf("Error() = %q, should contain both errors", errStr)
		}
	})
}
