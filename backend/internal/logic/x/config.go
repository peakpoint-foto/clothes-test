package x

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// AppConfig 服务端 X 应用配置（仅 OAuth 需要；Bearer 模式可不配）
type AppConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	APIBaseURL   string
	AuthBaseURL  string
	UploadURL    string
	Scopes       string
}

func loadAppConfig(ctx context.Context) (*AppConfig, error) {
	cfg := &AppConfig{
		ClientID:     strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.clientId", "").String()),
		ClientSecret: strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.clientSecret", "").String()),
		RedirectURI:  strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.redirectUri", "").String()),
		APIBaseURL:   strings.TrimSpace(g.Cfg().MustGet(ctx, "x.apiBaseURL", "https://api.x.com").String()),
		AuthBaseURL:  strings.TrimSpace(g.Cfg().MustGet(ctx, "x.authBaseURL", "https://twitter.com").String()),
		UploadURL:    strings.TrimSpace(g.Cfg().MustGet(ctx, "x.uploadURL", "https://upload.twitter.com/1.1/media/upload.json").String()),
		Scopes:       strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.scopes", "tweet.read tweet.write users.read offline.access media.write").String()),
	}

	if envKey := strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.clientIdEnv", "X_CLIENT_ID").String()); envKey != "" {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			cfg.ClientID = v
		}
	}
	if envKey := strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.clientSecretEnv", "X_CLIENT_SECRET").String()); envKey != "" {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			cfg.ClientSecret = v
		}
	}
	if envKey := strings.TrimSpace(g.Cfg().MustGet(ctx, "x.oauth.redirectUriEnv", "X_REDIRECT_URI").String()); envKey != "" {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			cfg.RedirectURI = v
		}
	}

	cfg.APIBaseURL = strings.TrimRight(cfg.APIBaseURL, "/")
	cfg.AuthBaseURL = strings.TrimRight(cfg.AuthBaseURL, "/")
	return cfg, nil
}

func (c *AppConfig) requireOAuth() error {
	if c.ClientID == "" {
		return fmt.Errorf("X OAuth 未配置 clientId（x.oauth.clientId 或环境变量 X_CLIENT_ID）")
	}
	if c.RedirectURI == "" {
		return fmt.Errorf("X OAuth 未配置 redirectUri（x.oauth.redirectUri 或环境变量 X_REDIRECT_URI）")
	}
	if err := validatePublicHTTPS(c.RedirectURI, true); err != nil {
		return fmt.Errorf("X OAuth redirectUri 无效: %w", err)
	}
	if err := validatePublicHTTPS(c.APIBaseURL, false); err != nil {
		return fmt.Errorf("X API Base URL 无效: %w", err)
	}
	if err := validatePublicHTTPS(c.AuthBaseURL, false); err != nil {
		return fmt.Errorf("X Auth Base URL 无效: %w", err)
	}
	return nil
}

// validatePublicHTTPS 要求非本机地址使用 HTTPS；允许 localhost HTTP。
func validatePublicHTTPS(raw string, allowPath bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("不是合法 URL")
	}
	host := strings.ToLower(u.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if loopback {
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("本机地址仅允许 http/https")
		}
	} else if u.Scheme != "https" {
		return fmt.Errorf("非本机地址必须使用 HTTPS")
	}
	if u.User != nil {
		return fmt.Errorf("不允许在 URL 中携带账号信息")
	}
	if !allowPath && (u.RawQuery != "" || u.Fragment != "") {
		return fmt.Errorf("Base URL 不能包含查询参数或片段")
	}
	return nil
}
