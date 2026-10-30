package x

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"

	v1 "backend/api/x/v1"
	logicx "backend/internal/logic/x"
)

func (c *ControllerV1) Post(ctx context.Context, req *v1.PostReq) (res *v1.PostRes, err error) {
	if req.Config == nil {
		return nil, gerror.New("缺少 x_config")
	}
	client, err := logicx.NewClientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	result, err := client.Post(ctx, logicx.PostInput{
		ImageB64: req.Image,
		MIME:     req.MIME,
		Text:     req.Text,
		Auth: logicx.AuthCredentials{
			Mode:        req.Config.AuthMode,
			AccessToken: req.Config.AccessToken,
		},
	})
	if err != nil {
		return nil, gerror.Wrap(err, "发布到 X 失败")
	}
	return &v1.PostRes{
		PostID:  result.PostID,
		URL:     result.URL,
		MediaID: result.MediaID,
	}, nil
}

func (c *ControllerV1) Verify(ctx context.Context, req *v1.VerifyReq) (res *v1.VerifyRes, err error) {
	if req.Config == nil {
		return nil, gerror.New("缺少 x_config")
	}
	client, err := logicx.NewClientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := client.Verify(ctx, logicx.AuthCredentials{
		Mode:        req.Config.AuthMode,
		AccessToken: req.Config.AccessToken,
	})
	if err != nil {
		return &v1.VerifyRes{OK: false, Message: err.Error()}, nil
	}
	return &v1.VerifyRes{
		OK:       true,
		UserID:   profile.ID,
		Username: profile.Username,
		Name:     profile.Name,
	}, nil
}

func (c *ControllerV1) OAuthStart(ctx context.Context, req *v1.OAuthStartReq) (res *v1.OAuthStartRes, err error) {
	authorizeURL, state, err := logicx.BuildAuthorizeURL(ctx, req.FrontendOrigin)
	if err != nil {
		return nil, gerror.Wrap(err, "无法启动 X OAuth")
	}
	return &v1.OAuthStartRes{
		AuthorizeURL: authorizeURL,
		State:        state,
	}, nil
}

func (c *ControllerV1) OAuthCallback(ctx context.Context, req *v1.OAuthCallbackReq) (res *v1.OAuthCallbackRes, err error) {
	r := ghttp.RequestFromCtx(ctx)
	if r == nil {
		return nil, gerror.New("缺少 HTTP 请求上下文")
	}

	if req.Error != "" {
		writeOAuthResultPage(r, false, nil, req.Error+": "+req.Desc, "")
		return &v1.OAuthCallbackRes{OK: false}, nil
	}
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.State) == "" {
		writeOAuthResultPage(r, false, nil, "缺少 code 或 state", "")
		return &v1.OAuthCallbackRes{OK: false}, nil
	}

	tokens, frontendOrigin, exchangeErr := logicx.ExchangeCode(ctx, req.Code, req.State)
	if exchangeErr != nil {
		writeOAuthResultPage(r, false, nil, exchangeErr.Error(), frontendOrigin)
		return &v1.OAuthCallbackRes{OK: false}, nil
	}

	payload := map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"token_type":    tokens.TokenType,
		"expires_in":    tokens.ExpiresIn,
		"scope":         tokens.Scope,
		"auth_mode":     "oauth",
	}
	writeOAuthResultPage(r, true, payload, "", frontendOrigin)
	return &v1.OAuthCallbackRes{OK: true}, nil
}

func writeOAuthResultPage(r *ghttp.Request, ok bool, payload map[string]any, errMsg, frontendOrigin string) {
	origin := strings.TrimSpace(frontendOrigin)
	// Never postMessage tokens to "*" or an empty target — that would allow opener spoofing.
	deliverSecrets := ok && origin != "" && origin != "*" && payload != nil
	if ok && !deliverSecrets {
		ok = false
		payload = nil
		if errMsg == "" {
			errMsg = "未授权的前端来源，已拒绝回传 Token"
		}
	}
	if !ok {
		payload = nil
	}

	type msg struct {
		Type    string         `json:"type"`
		OK      bool           `json:"ok"`
		Error   string         `json:"error,omitempty"`
		Payload map[string]any `json:"payload,omitempty"`
	}
	raw, _ := json.Marshal(msg{
		Type:    "kvpose-x-oauth",
		OK:      ok && deliverSecrets,
		Error:   errMsg,
		Payload: payload,
	})

	statusText := "X 授权成功，可以关闭此窗口。"
	if !ok || !deliverSecrets {
		if errMsg == "" {
			errMsg = "授权失败"
		}
		statusText = "X 授权失败：" + errMsg
	}

	postMessageJS := "/* no postMessage */"
	if deliverSecrets {
		originJSON, _ := json.Marshal(origin)
		postMessageJS = fmt.Sprintf(`
  var payload = %s;
  var target = %s;
  try {
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(payload, target);
    }
  } catch (e) {}
`, string(raw), string(originJSON))
	} else if origin != "" && origin != "*" {
		// Safe to notify failure to the known opener origin without secrets.
		failRaw, _ := json.Marshal(msg{Type: "kvpose-x-oauth", OK: false, Error: errMsg})
		originJSON, _ := json.Marshal(origin)
		postMessageJS = fmt.Sprintf(`
  var payload = %s;
  var target = %s;
  try {
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(payload, target);
    }
  } catch (e) {}
`, string(failRaw), string(originJSON))
	}

	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>X OAuth</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>body{font-family:system-ui,sans-serif;padding:24px;background:#0b0f14;color:#e8eef7}</style>
</head>
<body>
<p>%s</p>
<script>
(function(){
%s
  setTimeout(function(){ try { window.close(); } catch (e) {} }, 500);
})();
</script>
</body></html>`, html.EscapeString(statusText), postMessageJS)

	r.Response.ClearBuffer()
	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.Response.Header().Set("Cache-Control", "no-store")
	r.Response.WriteStatus(http.StatusOK, page)
	r.ExitAll()
}
