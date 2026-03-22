// Package token provides JWT-based token issuance, validation, refresh, and revocation.
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims extends jwt.RegisteredClaims with Relicta-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

// TokenPair holds an access token and its associated refresh token.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Config configures the token service.
type Config struct {
	// Secret is the HMAC-SHA256 signing key (32+ bytes recommended).
	Secret []byte
	// AccessTTL is the access token lifetime. Default: 15 minutes.
	AccessTTL time.Duration
	// RefreshTTL is the refresh token lifetime. Default: 24 hours.
	RefreshTTL time.Duration
	// Issuer is the token issuer claim. Default: "relicta".
	Issuer string
}

func (c *Config) defaults() {
	if c.AccessTTL == 0 {
		c.AccessTTL = 15 * time.Minute
	}
	if c.RefreshTTL == 0 {
		c.RefreshTTL = 24 * time.Hour
	}
	if c.Issuer == "" {
		c.Issuer = "relicta"
	}
}

// Service issues, validates, refreshes, and revokes JWT tokens.
type Service struct {
	cfg Config

	mu      sync.RWMutex
	revoked map[string]time.Time // jti -> expiry (for cleanup)
	done    chan struct{}         // closed on Close() to stop cleanup goroutine
}

// NewService creates a token service with the given config.
// A background goroutine periodically cleans expired revocation entries.
func NewService(cfg Config) (*Service, error) {
	if len(cfg.Secret) < 32 {
		return nil, errors.New("token: secret must be at least 32 bytes")
	}
	cfg.defaults()
	s := &Service{
		cfg:     cfg,
		revoked: make(map[string]time.Time),
		done:    make(chan struct{}),
	}
	go s.cleanupLoop()
	return s, nil
}

// Issue creates a new token pair for the given user.
func (s *Service) Issue(name string, roles []string) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(s.cfg.AccessTTL)

	accessJTI, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("token: generate access jti: %w", err)
	}

	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   name,
			ExpiresAt: jwt.NewNumericDate(accessExp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        accessJTI,
		},
		Name:  name,
		Roles: roles,
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("token: sign access token: %w", err)
	}

	refreshJTI, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("token: generate refresh jti: %w", err)
	}

	refreshClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   name,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.RefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        refreshJTI,
		},
		Name:  name,
		Roles: roles,
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(s.cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("token: sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExp,
	}, nil
}

// Validate parses and validates a token string, returning its claims.
// Returns an error if the token is expired, malformed, or revoked.
func (s *Service) Validate(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("token: unexpected signing method: %v", t.Header["alg"])
		}
		return s.cfg.Secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("token: %w", err)
	}
	if !tok.Valid {
		return nil, errors.New("token: invalid token")
	}

	if s.isRevoked(claims.ID) {
		return nil, errors.New("token: revoked")
	}

	return claims, nil
}

// Refresh validates a refresh token and issues a new token pair.
// The old refresh token is revoked.
func (s *Service) Refresh(refreshTokenStr string) (*TokenPair, error) {
	claims, err := s.Validate(refreshTokenStr)
	if err != nil {
		return nil, fmt.Errorf("token: invalid refresh token: %w", err)
	}

	s.Revoke(claims.ID, claims.ExpiresAt.Time)

	return s.Issue(claims.Name, claims.Roles)
}

// Revoke adds a token ID to the deny list until its expiry time.
func (s *Service) Revoke(jti string, expiresAt time.Time) {
	s.mu.Lock()
	s.revoked[jti] = expiresAt
	s.mu.Unlock()
}

// RevokeToken parses a token and revokes it by JTI.
func (s *Service) RevokeToken(tokenStr string) error {
	claims, err := s.Validate(tokenStr)
	if err != nil {
		return err
	}
	s.Revoke(claims.ID, claims.ExpiresAt.Time)
	return nil
}

// CleanExpired removes expired entries from the revocation list.
func (s *Service) CleanExpired() {
	now := time.Now()
	s.mu.Lock()
	for jti, exp := range s.revoked {
		if now.After(exp) {
			delete(s.revoked, jti)
		}
	}
	s.mu.Unlock()
}

// cleanupLoop runs periodic cleanup of expired revocation entries.
func (s *Service) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.CleanExpired()
		case <-s.done:
			return
		}
	}
}

// Close stops the background cleanup goroutine.
func (s *Service) Close() {
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
}

func (s *Service) isRevoked(jti string) bool {
	s.mu.RLock()
	_, ok := s.revoked[jti]
	s.mu.RUnlock()
	return ok
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
