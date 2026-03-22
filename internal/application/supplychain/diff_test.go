package supplychain

import (
	"testing"
)

func TestParseGoModDiff_VersionUpdate(t *testing.T) {
	oldMod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.3
	google.golang.org/grpc v1.60.0
)
`
	newMod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.4
	google.golang.org/grpc v1.61.0
)
`

	changes := ParseGoModDiff(oldMod, newMod)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}

	changeMap := make(map[string]DependencyChange)
	for _, c := range changes {
		changeMap[c.Name] = c
	}

	testify, ok := changeMap["github.com/stretchr/testify"]
	if !ok {
		t.Fatal("expected testify change")
	}
	if testify.ChangeType != ChangePatch {
		t.Errorf("expected patch change for testify, got %s", testify.ChangeType)
	}
	if testify.Ecosystem != "go" {
		t.Errorf("expected go ecosystem, got %s", testify.Ecosystem)
	}

	grpc, ok := changeMap["google.golang.org/grpc"]
	if !ok {
		t.Fatal("expected grpc change")
	}
	if grpc.ChangeType != ChangeMinor {
		t.Errorf("expected minor change for grpc, got %s", grpc.ChangeType)
	}
}

func TestParseGoModDiff_AddedDependency(t *testing.T) {
	oldMod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.3
)
`
	newMod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.3
	github.com/new/package v1.0.0
)
`

	changes := ParseGoModDiff(oldMod, newMod)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].Name != "github.com/new/package" {
		t.Errorf("expected new/package, got %s", changes[0].Name)
	}
	if changes[0].ChangeType != ChangeNew {
		t.Errorf("expected new change type, got %s", changes[0].ChangeType)
	}
	if changes[0].NewVersion != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", changes[0].NewVersion)
	}
}

func TestParseGoModDiff_RemovedDependency(t *testing.T) {
	oldMod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.3
	github.com/old/package v2.0.0
)
`
	newMod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.3
)
`

	changes := ParseGoModDiff(oldMod, newMod)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].Name != "github.com/old/package" {
		t.Errorf("expected old/package, got %s", changes[0].Name)
	}
	if changes[0].ChangeType != ChangeRemoved {
		t.Errorf("expected removed change type, got %s", changes[0].ChangeType)
	}
}

func TestParseGoModDiff_MajorUpdate(t *testing.T) {
	oldMod := `module github.com/example/app

go 1.22

require (
	google.golang.org/grpc v1.60.0
)
`
	newMod := `module github.com/example/app

go 1.22

require (
	google.golang.org/grpc v2.0.0
)
`

	changes := ParseGoModDiff(oldMod, newMod)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].ChangeType != ChangeMajor {
		t.Errorf("expected major change type, got %s", changes[0].ChangeType)
	}
}

func TestParseGoModDiff_EmptyInputs(t *testing.T) {
	changes := ParseGoModDiff("", "")
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for empty inputs, got %d", len(changes))
	}
}

func TestParseGoModDiff_SingleLineRequire(t *testing.T) {
	oldMod := `module github.com/example/app

go 1.22

require github.com/stretchr/testify v1.8.3
`
	newMod := `module github.com/example/app

go 1.22

require github.com/stretchr/testify v1.9.0
`

	changes := ParseGoModDiff(oldMod, newMod)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].ChangeType != ChangeMinor {
		t.Errorf("expected minor change, got %s", changes[0].ChangeType)
	}
}

func TestParseGoModDiff_NoChanges(t *testing.T) {
	mod := `module github.com/example/app

go 1.22

require (
	github.com/stretchr/testify v1.8.3
)
`

	changes := ParseGoModDiff(mod, mod)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes when content is identical, got %d", len(changes))
	}
}

func TestParsePackageJSONDiff_VersionUpdate(t *testing.T) {
	oldPkg := `{
  "name": "my-app",
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "^4.17.21"
  }
}`
	newPkg := `{
  "name": "my-app",
  "dependencies": {
    "express": "^4.19.0",
    "lodash": "^4.17.21"
  }
}`

	changes := ParsePackageJSONDiff(oldPkg, newPkg)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "express" {
		t.Errorf("expected express, got %s", changes[0].Name)
	}
	if changes[0].ChangeType != ChangeMinor {
		t.Errorf("expected minor change, got %s", changes[0].ChangeType)
	}
	if changes[0].Ecosystem != "npm" {
		t.Errorf("expected npm ecosystem, got %s", changes[0].Ecosystem)
	}
}

func TestParsePackageJSONDiff_AddedDependency(t *testing.T) {
	oldPkg := `{
  "dependencies": {
    "express": "^4.18.0"
  }
}`
	newPkg := `{
  "dependencies": {
    "express": "^4.18.0",
    "axios": "^1.6.0"
  }
}`

	changes := ParsePackageJSONDiff(oldPkg, newPkg)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "axios" {
		t.Errorf("expected axios, got %s", changes[0].Name)
	}
	if changes[0].ChangeType != ChangeNew {
		t.Errorf("expected new change type, got %s", changes[0].ChangeType)
	}
}

func TestParsePackageJSONDiff_RemovedDependency(t *testing.T) {
	oldPkg := `{
  "dependencies": {
    "express": "^4.18.0",
    "moment": "^2.29.0"
  }
}`
	newPkg := `{
  "dependencies": {
    "express": "^4.18.0"
  }
}`

	changes := ParsePackageJSONDiff(oldPkg, newPkg)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "moment" {
		t.Errorf("expected moment, got %s", changes[0].Name)
	}
	if changes[0].ChangeType != ChangeRemoved {
		t.Errorf("expected removed change type, got %s", changes[0].ChangeType)
	}
}

func TestParsePackageJSONDiff_DevDependencies(t *testing.T) {
	oldPkg := `{
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`
	newPkg := `{
  "devDependencies": {
    "jest": "^30.0.0"
  }
}`

	changes := ParsePackageJSONDiff(oldPkg, newPkg)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "jest" {
		t.Errorf("expected jest, got %s", changes[0].Name)
	}
	if changes[0].ChangeType != ChangeMajor {
		t.Errorf("expected major change for jest, got %s", changes[0].ChangeType)
	}
}

func TestParsePackageJSONDiff_EmptyInputs(t *testing.T) {
	changes := ParsePackageJSONDiff("", "")
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for empty inputs, got %d", len(changes))
	}
}

func TestParsePackageJSONDiff_InvalidJSON(t *testing.T) {
	changes := ParsePackageJSONDiff("not json", "also not json")
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for invalid JSON, got %d", len(changes))
	}
}

func TestParsePackageJSONDiff_NoChanges(t *testing.T) {
	pkg := `{
  "dependencies": {
    "express": "^4.18.0"
  }
}`

	changes := ParsePackageJSONDiff(pkg, pkg)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes when content is identical, got %d", len(changes))
	}
}

func TestClassifyVersionChange(t *testing.T) {
	tests := []struct {
		name       string
		oldVersion string
		newVersion string
		want       ChangeType
	}{
		{"patch bump", "v1.2.3", "v1.2.4", ChangePatch},
		{"minor bump", "v1.2.0", "v1.3.0", ChangeMinor},
		{"major bump", "v1.0.0", "v2.0.0", ChangeMajor},
		{"patch with prefix", "^1.2.3", "^1.2.4", ChangePatch},
		{"minor with tilde", "~1.2.0", "~1.3.0", ChangeMinor},
		{"pre-release ignored", "v1.2.3-beta.1", "v1.2.4", ChangePatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyVersionChange(
				stripSemverPrefix(tt.oldVersion),
				stripSemverPrefix(tt.newVersion),
			)
			if got != tt.want {
				t.Errorf("classifyVersionChange(%s, %s) = %s, want %s",
					tt.oldVersion, tt.newVersion, got, tt.want)
			}
		})
	}
}
