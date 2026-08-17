package config

// validate_persistence_test.go covers the load-path check on the persistence section.
//
// PersistenceConfig.Validate had held these rules since before ADR-013 and no load path
// called it: the only caller was persistence.NewEventStore, which has no production caller at
// all. So `backend: postgress` loaded without a word, read as "not postgres", and relicta
// wrote the team's audit trail to local files while they believed it was in their database.

import (
	"strings"
	"testing"
)

func TestTheDefaultConfigurationPassesPersistenceValidation(t *testing.T) {
	if err := Validate(DefaultConfig()); err != nil {
		t.Fatalf("the default configuration is invalid: %v", err)
	}
}

func TestAMisspelledBackendIsRefusedAtLoad(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Persistence.Backend = PersistenceBackend("postgress")

	err := Validate(cfg)

	if err == nil {
		t.Fatal("a misspelled persistence backend loaded without complaint; the operator " +
			"gets local files and every command reports success")
	}
	if !strings.Contains(err.Error(), "persistence") {
		t.Errorf("the error is %q and does not say which section is wrong", err)
	}
}

func TestTheSQLiteBackendNeedsNothingElseConfigured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Persistence.Backend = BackendSQLite

	if err := Validate(cfg); err != nil {
		t.Fatalf("`backend: sqlite` was rejected: %v — the store is a file relicta creates "+
			"in .relicta/, so there is nothing for the operator to supply", err)
	}
}

func TestPostgresWithoutAConnectionStringIsRefusedAtLoad(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Persistence.Backend = BackendPostgres

	err := Validate(cfg)

	if err == nil {
		t.Fatal("postgres was accepted with no connection string; the failure would surface " +
			"from inside a command instead of from the configuration that caused it")
	}
	if !strings.Contains(err.Error(), "connection_string") {
		t.Errorf("the error is %q and does not name the missing setting", err)
	}
}

// A Config assembled in code has no persistence section at all, and the field's zero value is
// not a backend anybody chose. Refusing it would fail every embedder that builds a Config
// without one.
func TestAConfigWithNoPersistenceSectionIsValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Persistence = PersistenceConfig{}

	if err := Validate(cfg); err != nil {
		t.Fatalf("a config with no persistence section was rejected: %v", err)
	}
}
