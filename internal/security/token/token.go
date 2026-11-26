// Package token provides JWT-based token issuance, validation, refresh, and revocation.
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims extends jwt.RegisteredClaims with Relicta-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	Name      string   `json:"name"`
	Roles     []string `json:"roles"`
	TokenUse  string   `json:"token_use,omitempty"`
	SessionID string   `json:"sid,omitempty"`
}

const (
	tokenUseAccess  = "access"
	tokenUseRefresh = "refresh"
)

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
	// Revocations stores token denials durably. When nil, revocations remain in memory.
	Revocations RevocationStore
}

// RevocationStore persists token denials and provides atomic refresh-token claims.
// Implementations must fail closed when storage is unavailable.
type RevocationStore interface {
	IsRevoked(jti string) (bool, error)
	Revoke(jti string, expiresAt time.Time) error
	ClaimRefresh(jti string, expiresAt time.Time) (bool, error)
	RevokePair(accessJTI string, accessExpiry time.Time, refreshJTI string, refreshExpiry time.Time) error
	CleanExpired(now time.Time) error
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
	cfg   Config
	store RevocationStore

	mu        sync.RWMutex
	revoked   map[string]time.Time // jti -> expiry (for cleanup)
	done      chan struct{}        // closed on Close() to stop cleanup goroutine
	closeOnce sync.Once
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
		store:   cfg.Revocations,
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
	sessionID, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("token: generate session id: %w", err)
	}

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
		Name:      name,
		Roles:     roles,
		TokenUse:  tokenUseAccess,
		SessionID: sessionID,
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
		Name:      name,
		Roles:     roles,
		TokenUse:  tokenUseRefresh,
		SessionID: sessionID,
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
	claims, err := s.parse(tokenStr)
	if err != nil {
		return nil, err
	}
	revoked, err := s.isRevoked(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("token: check revocation: %w", err)
	}
	if revoked {
		return nil, errors.New("token: revoked")
	}
	return claims, nil
}

// ValidateAccess validates a bearer access token and rejects refresh tokens.
// Untyped tokens from older Relicta versions are classified by their signed
// lifetime so access tokens continue to work during an upgrade.
func (s *Service) ValidateAccess(tokenStr string) (*Claims, error) {
	claims, err := s.Validate(tokenStr)
	if err != nil {
		return nil, err
	}
	if !s.hasTokenUse(claims, tokenUseAccess, s.cfg.AccessTTL) {
		return nil, errors.New("token: not an access token")
	}
	return claims, nil
}

func (s *Service) parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
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
	return claims, nil
}

// Refresh validates a refresh token and issues a new token pair.
// The old refresh token is revoked.
func (s *Service) Refresh(refreshTokenStr string) (*TokenPair, error) {
	claims, err := s.parse(refreshTokenStr)
	if err != nil {
		return nil, fmt.Errorf("token: invalid refresh token: %w", err)
	}
	if !s.hasTokenUse(claims, tokenUseRefresh, s.cfg.RefreshTTL) {
		return nil, errors.New("token: invalid refresh token: wrong token use")
	}

	// Claim the refresh token atomically. Without this check under the write
	// lock, concurrent requests could both validate and rotate the same token.
	if s.store != nil {
		claimed, err := s.store.ClaimRefresh(claims.ID, claims.ExpiresAt.Time)
		if err != nil {
			return nil, fmt.Errorf("token: claim refresh token: %w", err)
		}
		if !claimed {
			return nil, errors.New("token: refresh token revoked")
		}
	} else {
		s.mu.Lock()
		if _, revoked := s.revoked[claims.ID]; revoked {
			s.mu.Unlock()
			return nil, errors.New("token: refresh token revoked")
		}
		s.revoked[claims.ID] = claims.ExpiresAt.Time
		s.mu.Unlock()
	}

	return s.Issue(claims.Name, claims.Roles)
}

// Revoke adds a token ID to the deny list until its expiry time.
func (s *Service) Revoke(jti string, expiresAt time.Time) error {
	if s.store != nil {
		return s.store.Revoke(jti, expiresAt)
	}
	s.mu.Lock()
	s.revoked[jti] = expiresAt
	s.mu.Unlock()
	return nil
}

// RevokeToken parses a token and revokes it by JTI.
func (s *Service) RevokeToken(tokenStr string) error {
	claims, err := s.parse(tokenStr)
	if err != nil {
		return err
	}
	revoked, err := s.isRevoked(claims.ID)
	if err != nil {
		return fmt.Errorf("token: check revocation: %w", err)
	}
	if revoked {
		return errors.New("token: revoked")
	}
	return s.Revoke(claims.ID, claims.ExpiresAt.Time)
}

// RevokePair revokes a session's access and refresh tokens as one operation.
// Both tokens must be valid and belong to the same subject.
func (s *Service) RevokePair(accessToken, refreshToken string) error {
	access, err := s.parse(accessToken)
	if err != nil {
		return fmt.Errorf("token: invalid access token: %w", err)
	}
	refresh, err := s.parse(refreshToken)
	if err != nil {
		return fmt.Errorf("token: invalid refresh token: %w", err)
	}
	if access.Subject != refresh.Subject {
		return errors.New("token: access and refresh tokens belong to different subjects")
	}
	if !s.hasTokenUse(access, tokenUseAccess, s.cfg.AccessTTL) {
		return errors.New("token: first token is not an access token")
	}
	if !s.hasTokenUse(refresh, tokenUseRefresh, s.cfg.RefreshTTL) {
		return errors.New("token: second token is not a refresh token")
	}
	if access.SessionID != refresh.SessionID {
		return errors.New("token: access and refresh tokens belong to different sessions")
	}

	if s.store != nil {
		if err := s.store.RevokePair(access.ID, access.ExpiresAt.Time, refresh.ID, refresh.ExpiresAt.Time); err != nil {
			return fmt.Errorf("token: revoke session: %w", err)
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[access.ID] = access.ExpiresAt.Time
	s.revoked[refresh.ID] = refresh.ExpiresAt.Time
	return nil
}

func (s *Service) hasTokenUse(claims *Claims, expected string, legacyTTL time.Duration) bool {
	if claims.TokenUse != "" {
		return claims.TokenUse == expected
	}
	if expected == tokenUseRefresh && s.cfg.AccessTTL == s.cfg.RefreshTTL {
		return false
	}
	// JWT NumericDate serializes at second precision. A small tolerance keeps
	// older tokens valid while still distinguishing the 15-minute access
	// lifetime from the longer refresh lifetime.
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return false
	}
	lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	if lifetime < legacyTTL {
		return legacyTTL-lifetime <= 2*time.Second
	}
	return lifetime-legacyTTL <= 2*time.Second
}

// CleanExpired removes expired entries from the revocation list.
func (s *Service) CleanExpired() {
	now := time.Now()
	if s.store != nil {
		if err := s.store.CleanExpired(now); err != nil {
			slog.Warn("clean expired token revocations", "error", err)
		}
		return
	}
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
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

func (s *Service) isRevoked(jti string) (bool, error) {
	if s.store != nil {
		return s.store.IsRevoked(jti)
	}
	s.mu.RLock()
	_, ok := s.revoked[jti]
	s.mu.RUnlock()
	return ok, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
