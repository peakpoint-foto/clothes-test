package x

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

const oauthStateTTL = 10 * time.Minute

type oauthPending struct {
	CodeVerifier   string
	FrontendOrigin string
	ExpiresAt      time.Time
}

type oauthStore struct {
	mu   sync.Mutex
	byID map[string]oauthPending
}

var globalOAuthStore = &oauthStore{byID: make(map[string]oauthPending)}

func (s *oauthStore) put(state, verifier, frontendOrigin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.byID[state] = oauthPending{
		CodeVerifier:   verifier,
		FrontendOrigin: frontendOrigin,
		ExpiresAt:      time.Now().Add(oauthStateTTL),
	}
}

func (s *oauthStore) take(state string) (oauthPending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	item, ok := s.byID[state]
	if !ok {
		return oauthPending{}, false
	}
	delete(s.byID, state)
	if time.Now().After(item.ExpiresAt) {
		return oauthPending{}, false
	}
	return item, true
}

func (s *oauthStore) cleanupLocked(now time.Time) {
	for k, v := range s.byID {
		if now.After(v.ExpiresAt) {
			delete(s.byID, k)
		}
	}
}

func randomURLSafe(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func newPKCEPair() (verifier, challenge string, err error) {
	verifier, err = randomURLSafe(32)
	if err != nil {
		return "", "", err
	}
	if len(verifier) < 43 {
		return "", "", fmt.Errorf("PKCE verifier too short")
	}
	return verifier, pkceChallengeS256(verifier), nil
}
