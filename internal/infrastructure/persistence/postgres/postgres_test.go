package postgres_test

import (
	"os"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres/migrations"
)

func TestMigrationFiles_Embedded(t *testing.T) {
	t.Parallel()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded migrations: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no migration files found in embedded FS")
	}

	// Verify we have matching up/down pairs.
	upFiles := make(map[string]bool)
	downFiles := make(map[string]bool)

	for _, entry := range entries {
		name := entry.Name()
		if !isSQL(name) {
			continue
		}
		if isUpMigration(name) {
			upFiles[migrationVersion(name)] = true
		} else if isDownMigration(name) {
			downFiles[migrationVersion(name)] = true
		}
	}

	for version := range upFiles {
		if !downFiles[version] {
			t.Errorf("migration %s has up.sql but no down.sql", version)
		}
	}
	for version := range downFiles {
		if !upFiles[version] {
			t.Errorf("migration %s has down.sql but no up.sql", version)
		}
	}
}

func TestMigrationFiles_ValidSQL(t *testing.T) {
	t.Parallel()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded migrations: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isSQL(entry.Name()) {
			continue
		}

		data, err := migrations.FS.ReadFile(entry.Name())
		if err != nil {
			t.Errorf("reading %s: %v", entry.Name(), err)
			continue
		}

		content := string(data)
		if len(content) == 0 {
			t.Errorf("migration file %s is empty", entry.Name())
		}

		// Basic SQL validation: should contain a SQL keyword.
		if !containsSQLKeyword(content) {
			t.Errorf("migration file %s does not contain recognizable SQL", entry.Name())
		}
	}
}

// --- Config Tests ---

func TestPersistenceConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultPersistenceConfig()

	if cfg.Backend != config.BackendFile {
		t.Errorf("default Backend = %q, want %q", cfg.Backend, config.BackendFile)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("default PoolSize = %d, want 10", cfg.PoolSize)
	}
	if cfg.MigrationMode != config.MigrationModeManual {
		t.Errorf("default MigrationMode = %q, want %q", cfg.MigrationMode, config.MigrationModeManual)
	}
	if cfg.FilePath != ".relicta/events" {
		t.Errorf("default FilePath = %q, want %q", cfg.FilePath, ".relicta/events")
	}
}

func TestPersistenceConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     config.PersistenceConfig
		wantErr bool
	}{
		{
			name:    "valid file backend",
			cfg:     config.PersistenceConfig{Backend: config.BackendFile},
			wantErr: false,
		},
		{
			name: "valid postgres backend",
			cfg: config.PersistenceConfig{
				Backend:          config.BackendPostgres,
				ConnectionString: "postgres://localhost:5432/test",
				PoolSize:         5,
				MigrationMode:    config.MigrationModeAuto,
			},
			wantErr: false,
		},
		{
			name: "postgres without connection string",
			cfg: config.PersistenceConfig{
				Backend:       config.BackendPostgres,
				PoolSize:      5,
				MigrationMode: config.MigrationModeManual,
			},
			wantErr: true,
		},
		{
			name: "postgres with zero pool size",
			cfg: config.PersistenceConfig{
				Backend:          config.BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         0,
				MigrationMode:    config.MigrationModeManual,
			},
			wantErr: true,
		},
		{
			name: "postgres with invalid migration mode",
			cfg: config.PersistenceConfig{
				Backend:          config.BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         5,
				MigrationMode:    "invalid",
			},
			wantErr: true,
		},
		{
			name:    "invalid backend",
			cfg:     config.PersistenceConfig{Backend: "redis"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpandEnvVars(t *testing.T) {
	t.Setenv("TEST_DB_URL", "postgres://user:pass@host:5432/db")
	t.Setenv("TEST_HOST", "myhost")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single var",
			input: "${TEST_DB_URL}",
			want:  "postgres://user:pass@host:5432/db",
		},
		{
			name:  "var in string",
			input: "postgresql://${TEST_HOST}:5432/mydb",
			want:  "postgresql://myhost:5432/mydb",
		},
		{
			name:  "missing var unchanged",
			input: "${NONEXISTENT_VAR_12345}",
			want:  "${NONEXISTENT_VAR_12345}",
		},
		{
			name:  "no vars",
			input: "postgres://localhost:5432/db",
			want:  "postgres://localhost:5432/db",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ExpandEnvVars(tt.input)
			if got != tt.want {
				t.Errorf("ExpandEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Helper Functions ---

func isSQL(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == ".sql"
}

func isUpMigration(name string) bool {
	return len(name) > 7 && name[len(name)-7:] == ".up.sql"
}

func isDownMigration(name string) bool {
	return len(name) > 9 && name[len(name)-9:] == ".down.sql"
}

func migrationVersion(name string) string {
	for i, c := range name {
		if c == '_' {
			return name[:i]
		}
	}
	return name
}

func containsSQLKeyword(content string) bool {
	keywords := []string{"CREATE", "DROP", "INSERT", "SELECT", "ALTER", "DELETE", "UPDATE", "TABLE", "INDEX"}
	upper := toUpper(content)
	for _, kw := range keywords {
		if containsStr(upper, kw) {
			return true
		}
	}
	return false
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Suppress unused import warnings for os (used indirectly via t.Setenv).
var _ = os.Getenv
