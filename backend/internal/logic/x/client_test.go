package x

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidatePublicHTTPS(t *testing.T) {
	if err := validatePublicHTTPS("https://api.x.com", false); err != nil {
		t.Fatalf("expected https ok: %v", err)
	}
	if err := validatePublicHTTPS("http://api.x.com", false); err == nil {
		t.Fatal("expected remote http rejected")
	}
	if err := validatePublicHTTPS("http://127.0.0.1:8000/x/oauth/callback", true); err != nil {
		t.Fatalf("expected loopback http ok: %v", err)
	}
	if err := validatePublicHTTPS("https://evil.com/path?x=1", false); err == nil {
		t.Fatal("expected query rejected on base url")
	}
}

func TestSanitizePostTextWeighted(t *testing.T) {
	if got := sanitizePostText("  "); got != "AI try-on result" {
		t.Fatalf("empty default: %q", got)
	}
	// 140 CJK => 280 weighted units, still allowed
	okCJK := strings.Repeat("测", 140)
	if got := sanitizePostText(okCJK); got != okCJK {
		t.Fatalf("140 CJK should pass unchanged, got len=%d weight=%d", len([]rune(got)), weightedLen(got))
	}
	// 141 CJK exceeds 280; must truncate with ellipsis and stay within limit
	long := strings.Repeat("测", 141)
	got := sanitizePostText(long)
	if weightedLen(got) > defaultMaxWeightedUnits {
		t.Fatalf("weighted length %d exceeds %d", weightedLen(got), defaultMaxWeightedUnits)
	}
	if !strings.HasSuffix(got, postTextEllipsis) {
		t.Fatalf("expected ellipsis suffix: %q", got)
	}
	// ASCII near limit
	ascii := strings.Repeat("a", 300)
	gotASCII := sanitizePostText(ascii)
	if weightedLen(gotASCII) > defaultMaxWeightedUnits {
		t.Fatalf("ascii weighted %d", weightedLen(gotASCII))
	}
}

func TestDecodeImagePayload(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 1, 2, 3}
	b64 := base64.StdEncoding.EncodeToString(png)
	data, mime, err := decodeImagePayload(b64, "")
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" {
		t.Fatalf("mime=%s", mime)
	}
	if len(data) != len(png) {
		t.Fatalf("len=%d", len(data))
	}

	dataURL := "data:image/png;base64," + b64
	_, mime, err = decodeImagePayload(dataURL, "")
	if err != nil || mime != "image/png" {
		t.Fatalf("data url: mime=%s err=%v", mime, err)
	}
}

func TestPKCEAndOAuthStore(t *testing.T) {
	verifier, challenge, err := newPKCEPair()
	if err != nil {
		t.Fatal(err)
	}
	if challenge != pkceChallengeS256(verifier) {
		t.Fatal("challenge mismatch")
	}
	state, err := randomURLSafe(16)
	if err != nil {
		t.Fatal(err)
	}
	globalOAuthStore.put(state, verifier, "https://paipiange.github.io")
	got, ok := globalOAuthStore.take(state)
	if !ok || got.CodeVerifier != verifier || got.FrontendOrigin != "https://paipiange.github.io" {
		t.Fatalf("store take failed: %+v ok=%v", got, ok)
	}
	if _, ok := globalOAuthStore.take(state); ok {
		t.Fatal("state should be one-time")
	}
}

func TestClientVerifyAndPost(t *testing.T) {
	var sawAuth string
	var tweetBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/2/users/me", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"id": "42", "username": "demo", "name": "Demo"},
		})
	})
	mux.HandleFunc("/1.1/media/upload.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"media_id_string": "media-1"})
	})
	mux.HandleFunc("/2/tweets", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&tweetBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"id": "post-9"}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(&AppConfig{
		APIBaseURL: srv.URL,
		UploadURL:  srv.URL + "/1.1/media/upload.json",
	})
	profile, err := client.Verify(context.Background(), AuthCredentials{Mode: "bearer", AccessToken: "tok-abc"})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Username != "demo" || sawAuth != "Bearer tok-abc" {
		t.Fatalf("verify mismatch: %+v auth=%s", profile, sawAuth)
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 1, 2, 3}
	result, err := client.Post(context.Background(), PostInput{
		ImageB64: base64.StdEncoding.EncodeToString(png),
		Text:     "hello try-on",
		Auth:     AuthCredentials{Mode: "oauth", AccessToken: "tok-abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PostID != "post-9" || result.MediaID != "media-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if tweetBody["text"] != "hello try-on" {
		t.Fatalf("tweet body: %#v", tweetBody)
	}
}

func TestValidateAuthRejectsEmpty(t *testing.T) {
	if err := validateAuth(AuthCredentials{Mode: "bearer", AccessToken: ""}); err == nil {
		t.Fatal("expected empty token error")
	}
	if err := validateAuth(AuthCredentials{Mode: "basic", AccessToken: "x"}); err == nil {
		t.Fatal("expected bad mode error")
	}
}

func TestValidateTrustedFrontendOrigin(t *testing.T) {
	allow := []string{"https://paipiange.github.io", "http://localhost:3000"}

	if err := validateTrustedFrontendOrigin("https://paipiange.github.io", allow); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedFrontendOrigin("http://127.0.0.1:5500", allow); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedFrontendOrigin("http://localhost:9999", nil); err != nil {
		t.Fatal(err)
	}
	// Attacker HTTPS must NOT be trusted merely for being HTTPS
	if err := validateTrustedFrontendOrigin("https://attacker.example", allow); err == nil {
		t.Fatal("attacker https must be rejected")
	}
	if err := validateTrustedFrontendOrigin("https://evil.com", nil); err == nil {
		t.Fatal("unlisted https must be rejected even with empty allowlist")
	}
	if err := validateTrustedFrontendOrigin("http://evil.com", allow); err == nil {
		t.Fatal("remote http should fail")
	}
	if err := validateTrustedFrontendOrigin("", allow); err == nil {
		t.Fatal("empty origin should fail")
	}
	if err := validateTrustedFrontendOrigin("https://paipiange.github.io/path", allow); err == nil {
		t.Fatal("origin with path should fail")
	}
}
