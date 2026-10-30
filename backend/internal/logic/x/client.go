package x

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"backend/internal/imagevalidate"
)

const (
	defaultMaxTextRunes = 260
	httpTimeout         = 45 * time.Second
)

// AuthCredentials 单次请求使用的用户凭据（不持久化）
type AuthCredentials struct {
	Mode        string // bearer | oauth
	AccessToken string
}

// PostInput 发帖入参
type PostInput struct {
	ImageB64 string
	MIME     string
	Text     string
	Auth     AuthCredentials
}

// PostResult 发帖结果
type PostResult struct {
	PostID  string
	URL     string
	MediaID string
}

// UserProfile users/me 摘要
type UserProfile struct {
	ID       string
	Username string
	Name     string
}

type Client struct {
	cfg        *AppConfig
	httpClient *http.Client
}

func NewClient(cfg *AppConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

func (c *Client) Verify(ctx context.Context, auth AuthCredentials) (*UserProfile, error) {
	if err := validateAuth(auth); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.APIBaseURL+"/2/users/me?user.fields=id,name,username", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接 X API")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("X Token 校验失败（HTTP %d）", resp.StatusCode)
	}
	var parsed struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Data.ID == "" {
		return nil, fmt.Errorf("X 返回了无法解析的用户信息")
	}
	return &UserProfile{
		ID:       parsed.Data.ID,
		Username: parsed.Data.Username,
		Name:     parsed.Data.Name,
	}, nil
}

func (c *Client) Post(ctx context.Context, in PostInput) (*PostResult, error) {
	if err := validateAuth(in.Auth); err != nil {
		return nil, err
	}
	imgBytes, mime, err := decodeImagePayload(in.ImageB64, in.MIME)
	if err != nil {
		return nil, err
	}
	mediaID, err := c.uploadMedia(ctx, in.Auth.AccessToken, imgBytes, mime)
	if err != nil {
		return nil, err
	}
	text := sanitizePostText(in.Text)
	postID, err := c.createTweet(ctx, in.Auth.AccessToken, text, mediaID)
	if err != nil {
		return nil, err
	}
	return &PostResult{
		PostID:  postID,
		URL:     "https://x.com/i/status/" + postID,
		MediaID: mediaID,
	}, nil
}

func validateAuth(auth AuthCredentials) error {
	mode := strings.ToLower(strings.TrimSpace(auth.Mode))
	if mode != "bearer" && mode != "oauth" {
		return fmt.Errorf("不支持的 X 鉴权方式")
	}
	token := strings.TrimSpace(auth.AccessToken)
	if token == "" {
		return fmt.Errorf("缺少 X Access Token")
	}
	if strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("X Access Token 格式无效")
	}
	return nil
}

func decodeImagePayload(raw, declaredMIME string) ([]byte, string, error) {
	_, b64, err := imagevalidate.ParseDataURL(raw)
	if err != nil {
		return nil, "", err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		// 兼容 URL-safe / 无 padding
		data, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			return nil, "", fmt.Errorf("图片 base64 无效")
		}
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("图片为空")
	}
	if len(data) > 5<<20 {
		return nil, "", fmt.Errorf("图片过大，请先压缩后再分享到 X")
	}
	mime := strings.ToLower(strings.TrimSpace(declaredMIME))
	if mime == "" {
		mime = sniffImageMIME(data)
	}
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, "", fmt.Errorf("仅支持 JPEG、PNG、WebP 分享到 X")
	}
	return data, mime, nil
}

func sniffImageMIME(data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

func sanitizePostText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		trimmed = "AI try-on result"
	}
	if utf8.RuneCountInString(trimmed) <= defaultMaxTextRunes {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:defaultMaxTextRunes-1]) + "…"
}

func (c *Client) uploadMedia(ctx context.Context, token string, img []byte, mime string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("media", filenameForMIME(mime))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(img); err != nil {
		return "", err
	}
	_ = w.WriteField("media_category", "tweet_image")
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.UploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("上传媒体到 X 失败")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("X 媒体上传失败（HTTP %d）", resp.StatusCode)
	}
	var parsed struct {
		MediaIDString string `json:"media_id_string"`
		MediaID       int64  `json:"media_id"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("X 媒体上传响应无法解析")
	}
	if parsed.MediaIDString != "" {
		return parsed.MediaIDString, nil
	}
	if parsed.MediaID != 0 {
		return fmt.Sprintf("%d", parsed.MediaID), nil
	}
	return "", fmt.Errorf("X 未返回 media_id")
}

func filenameForMIME(mime string) string {
	switch mime {
	case "image/png":
		return "tryon.png"
	case "image/webp":
		return "tryon.webp"
	default:
		return "tryon.jpg"
	}
}

func (c *Client) createTweet(ctx context.Context, token, text, mediaID string) (string, error) {
	payload := map[string]any{
		"text": text,
		"media": map[string]any{
			"media_ids": []string{mediaID},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIBaseURL+"/2/tweets", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发布帖文到 X 失败")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("X 发帖失败（HTTP %d）", resp.StatusCode)
	}
	var parsed struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Data.ID == "" {
		return "", fmt.Errorf("X 发帖响应无法解析")
	}
	return parsed.Data.ID, nil
}

// BuildAuthorizeURL 生成 PKCE 授权 URL，并把 verifier 暂存到内存。
func BuildAuthorizeURL(ctx context.Context, frontendOrigin string) (authorizeURL, state string, err error) {
	cfg, err := loadAppConfig(ctx)
	if err != nil {
		return "", "", err
	}
	if err := cfg.requireOAuth(); err != nil {
		return "", "", err
	}
	if frontendOrigin != "" {
		if err := validateFrontendOrigin(frontendOrigin); err != nil {
			return "", "", err
		}
	}
	verifier, challenge, err := newPKCEPair()
	if err != nil {
		return "", "", err
	}
	state, err = randomURLSafe(24)
	if err != nil {
		return "", "", err
	}
	globalOAuthStore.put(state, verifier, frontendOrigin)

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("scope", cfg.Scopes)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	return cfg.AuthBaseURL + "/i/oauth2/authorize?" + q.Encode(), state, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// ExchangeCode 用授权码换取用户 Access Token（结果返回给前端，后端不落盘）。
func ExchangeCode(ctx context.Context, code, state string) (tokens *tokenResponse, frontendOrigin string, err error) {
	cfg, err := loadAppConfig(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := cfg.requireOAuth(); err != nil {
		return nil, "", err
	}
	pending, ok := globalOAuthStore.take(state)
	if !ok {
		return nil, "", fmt.Errorf("无效或过期的 OAuth state")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURI)
	form.Set("code_verifier", pending.CodeVerifier)
	form.Set("client_id", cfg.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIBaseURL+"/2/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if cfg.ClientSecret != "" {
		basic := base64.StdEncoding.EncodeToString([]byte(cfg.ClientID + ":" + cfg.ClientSecret))
		req.Header.Set("Authorization", "Basic "+basic)
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("无法连接 X Token 端点")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("X OAuth 换码失败（HTTP %d）", resp.StatusCode)
	}
	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.AccessToken == "" {
		return nil, "", fmt.Errorf("X OAuth 响应无法解析")
	}
	return &parsed, pending.FrontendOrigin, nil
}

func validateFrontendOrigin(origin string) error {
	u, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("frontend_origin 无效")
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("frontend_origin 不能包含路径")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("frontend_origin 无效")
	}
	host := strings.ToLower(u.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if loopback {
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("frontend_origin 协议无效")
		}
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("非本机 frontend_origin 必须使用 HTTPS")
	}
	return nil
}

// NewClientFromContext 读取配置并构造客户端。
func NewClientFromContext(ctx context.Context) (*Client, error) {
	cfg, err := loadAppConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := validatePublicHTTPS(cfg.APIBaseURL, false); err != nil {
		return nil, fmt.Errorf("X API Base URL 无效: %w", err)
	}
	if err := validatePublicHTTPS(cfg.UploadURL, true); err != nil {
		return nil, fmt.Errorf("X Upload URL 无效: %w", err)
	}
	return NewClient(cfg), nil
}
