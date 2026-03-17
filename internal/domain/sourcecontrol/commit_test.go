// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"testing"
	"time"
)

func TestCommitHash_Short(t *testing.T) {
	tests := []struct {
		name string
		hash CommitHash
		want string
	}{
		{"full hash", CommitHash("abc1234567890def"), "abc1234"},
		{"exactly 7 chars", CommitHash("abc1234"), "abc1234"},
		{"less than 7 chars", CommitHash("abc12"), "abc12"},
		{"empty hash", CommitHash(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.Short(); got != tt.want {
				t.Errorf("CommitHash.Short() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommitHash_String(t *testing.T) {
	hash := CommitHash("abc1234567890def")
	if got := hash.String(); got != "abc1234567890def" {
		t.Errorf("CommitHash.String() = %v, want abc1234567890def", got)
	}
}

func TestCommitHash_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		hash CommitHash
		want bool
	}{
		{"empty hash", CommitHash(""), true},
		{"non-empty hash", CommitHash("abc123"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.IsEmpty(); got != tt.want {
				t.Errorf("CommitHash.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCommit(t *testing.T) {
	hash := CommitHash("abc1234567890def")
	message := "feat: add new feature\n\nThis is the body"
	author := Author{Name: "John Doe", Email: "john@example.com"}
	date := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	commit := NewCommit(hash, message, author, date)

	if commit.Hash() != hash {
		t.Errorf("Hash() = %v, want %v", commit.Hash(), hash)
	}
	if commit.Message() != message {
		t.Errorf("Message() = %v, want %v", commit.Message(), message)
	}
	if commit.Author() != author {
		t.Errorf("Author() = %v, want %v", commit.Author(), author)
	}
	if !commit.Date().Equal(date) {
		t.Errorf("Date() = %v, want %v", commit.Date(), date)
	}
}

func TestCommit_ShortHash(t *testing.T) {
	commit := NewCommit(
		CommitHash("abc1234567890def"),
		"test message",
		Author{Name: "Test", Email: "test@test.com"},
		time.Now(),
	)

	if got := commit.ShortHash(); got != "abc1234" {
		t.Errorf("ShortHash() = %v, want abc1234", got)
	}
}

func TestCommit_Subject(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "single line message",
			message: "feat: add new feature",
			want:    "feat: add new feature",
		},
		{
			name:    "multi-line message",
			message: "feat: add new feature\n\nThis is the body\nWith multiple lines",
			want:    "feat: add new feature",
		},
		{
			name:    "empty message",
			message: "",
			want:    "",
		},
		{
			name:    "message with only newline",
			message: "\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commit := NewCommit(
				CommitHash("abc123"),
				tt.message,
				Author{Name: "Test", Email: "test@test.com"},
				time.Now(),
			)
			if got := commit.Subject(); got != tt.want {
				t.Errorf("Subject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommit_Committer(t *testing.T) {
	commit := NewCommit(
		CommitHash("abc123"),
		"test message",
		Author{Name: "Author", Email: "author@test.com"},
		time.Now(),
	)

	// Initially committer is empty
	if commit.Committer().Name != "" {
		t.Error("Committer should be empty initially")
	}

	// Set committer
	committer := Author{Name: "Committer", Email: "committer@test.com"}
	commit.SetCommitter(committer)

	if commit.Committer() != committer {
		t.Errorf("Committer() = %v, want %v", commit.Committer(), committer)
	}
}

func TestCommit_Parents(t *testing.T) {
	commit := NewCommit(
		CommitHash("abc123"),
		"test message",
		Author{Name: "Test", Email: "test@test.com"},
		time.Now(),
	)

	// Initially parents is nil
	if commit.Parents() != nil {
		t.Error("Parents should be nil initially")
	}

	// Set parents
	parents := []CommitHash{"parent1", "parent2"}
	commit.SetParents(parents)

	if len(commit.Parents()) != 2 {
		t.Errorf("Parents() length = %v, want 2", len(commit.Parents()))
	}
	if commit.Parents()[0] != "parent1" {
		t.Errorf("Parents()[0] = %v, want parent1", commit.Parents()[0])
	}
}

func TestCommit_IsMergeCommit(t *testing.T) {
	tests := []struct {
		name    string
		parents []CommitHash
		want    bool
	}{
		{"no parents", nil, false},
		{"single parent", []CommitHash{"parent1"}, false},
		{"two parents (merge)", []CommitHash{"parent1", "parent2"}, true},
		{"multiple parents", []CommitHash{"p1", "p2", "p3"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commit := NewCommit(
				CommitHash("abc123"),
				"test",
				Author{},
				time.Now(),
			)
			commit.SetParents(tt.parents)

			if got := commit.IsMergeCommit(); got != tt.want {
				t.Errorf("IsMergeCommit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommit_TreeHash(t *testing.T) {
	commit := NewCommit(
		CommitHash("abc123"),
		"test message",
		Author{Name: "Test", Email: "test@test.com"},
		time.Now(),
	)

	// Initially tree hash is empty
	if commit.TreeHash() != "" {
		t.Error("TreeHash should be empty initially")
	}

	// Set tree hash
	commit.SetTreeHash("tree1234567890")

	if commit.TreeHash() != "tree1234567890" {
		t.Errorf("TreeHash() = %v, want tree1234567890", commit.TreeHash())
	}
}

func TestAuthor_Fields(t *testing.T) {
	author := Author{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	if author.Name != "John Doe" {
		t.Errorf("Author.Name = %v, want John Doe", author.Name)
	}
	if author.Email != "john@example.com" {
		t.Errorf("Author.Email = %v, want john@example.com", author.Email)
	}
}
