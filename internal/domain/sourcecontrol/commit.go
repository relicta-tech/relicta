// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"time"
)

// CommitHash represents a git commit hash.
type CommitHash string

// Short returns the short (7 character) hash.
func (h CommitHash) Short() string {
	if len(h) > 7 {
		return string(h[:7])
	}
	return string(h)
}

// String returns the full hash.
func (h CommitHash) String() string {
	return string(h)
}

// IsEmpty returns true if the hash is empty.
func (h CommitHash) IsEmpty() bool {
	return h == ""
}

// Commit represents a git commit entity.
type Commit struct {
	hash      CommitHash
	message   string
	author    Author
	committer Author
	date      time.Time
	parents   []CommitHash
	treeHash  string
}

// Author represents a commit author or committer.
type Author struct {
	Name  string
	Email string
}

// NewCommit creates a new Commit entity.
func NewCommit(hash CommitHash, message string, author Author, date time.Time) *Commit {
	return &Commit{
		hash:    hash,
		message: message,
		author:  author,
		date:    date,
	}
}

// Hash returns the commit hash.
func (c *Commit) Hash() CommitHash {
	return c.hash
}

// ShortHash returns the short commit hash.
func (c *Commit) ShortHash() string {
	return c.hash.Short()
}

// Message returns the full commit message.
func (c *Commit) Message() string {
	return c.message
}

// Subject returns the first line of the commit message.
func (c *Commit) Subject() string {
	for i, r := range c.message {
		if r == '\n' {
			return c.message[:i]
		}
	}
	return c.message
}

// Author returns the commit author.
func (c *Commit) Author() Author {
	return c.author
}

// Committer returns the committer.
func (c *Commit) Committer() Author {
	return c.committer
}

// SetCommitter sets the committer.
func (c *Commit) SetCommitter(committer Author) {
	c.committer = committer
}

// Date returns the commit date.
func (c *Commit) Date() time.Time {
	return c.date
}

// Parents returns the parent commit hashes.
func (c *Commit) Parents() []CommitHash {
	return c.parents
}

// SetParents sets the parent hashes.
func (c *Commit) SetParents(parents []CommitHash) {
	c.parents = parents
}

// IsMergeCommit returns true if this is a merge commit.
func (c *Commit) IsMergeCommit() bool {
	return len(c.parents) > 1
}

// TreeHash returns the tree hash.
func (c *Commit) TreeHash() string {
	return c.treeHash
}

// SetTreeHash sets the tree hash.
func (c *Commit) SetTreeHash(hash string) {
	c.treeHash = hash
}
