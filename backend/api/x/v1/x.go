package v1

import "github.com/gogf/gf/v2/frame/g"

// ClientXConfig 浏览器本地保存的 X 凭据（仅随本次请求进入后端内存，不落盘）。
type ClientXConfig struct {
	// AuthMode: bearer（API Key / Bearer Token）或 oauth（用户 OAuth Access Token）
	AuthMode    string `json:"auth_mode" v:"required|in:bearer,oauth" dc:"鉴权方式：bearer / oauth"`
	AccessToken string `json:"access_token" v:"required" dc:"X API Bearer 或 OAuth Access Token（敏感，仅本次请求使用）"`
}

// PostReq 将试穿结果图发到 X
type PostReq struct {
	g.Meta `path:"/x/post" method:"post" tags:"X" summary:"将试穿结果发布到 X"`
	Image  string         `json:"image" v:"required" dc:"试穿结果图 base64 或 data URL"`
	MIME   string         `json:"mime,omitempty" dc:"图片 MIME（可选；缺省时按文件签名检测）"`
	Text   string         `json:"text,omitempty" dc:"帖文正文（最长 280 字符量级，后端会截断）"`
	Config *ClientXConfig `json:"x_config" v:"required" dc:"浏览器临时提供的 X 凭据"`
}

// PostRes 发布结果
type PostRes struct {
	PostID  string `json:"post_id" dc:"X 帖文 ID"`
	URL     string `json:"url" dc:"帖文链接"`
	MediaID string `json:"media_id,omitempty" dc:"上传的媒体 ID"`
}

// VerifyReq 校验当前 Token 是否可用（调用 users/me）
type VerifyReq struct {
	g.Meta `path:"/x/verify" method:"post" tags:"X" summary:"校验 X Access Token"`
	Config *ClientXConfig `json:"x_config" v:"required" dc:"浏览器临时提供的 X 凭据"`
}

// VerifyRes Token 校验结果
type VerifyRes struct {
	OK       bool   `json:"ok"`
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	Message  string `json:"message,omitempty"`
}

// OAuthStartReq 启动 OAuth 2.0 PKCE 授权
type OAuthStartReq struct {
	g.Meta `path:"/x/oauth/start" method:"get" tags:"X" summary:"启动 X OAuth 2.0 PKCE"`
	// 可选：前端 origin，用于回调页 postMessage 目标校验
	FrontendOrigin string `json:"frontend_origin" in:"query" dc:"前端页面 Origin（用于回调 postMessage）"`
}

// OAuthStartRes 返回授权 URL（前端打开或跳转）
type OAuthStartRes struct {
	AuthorizeURL string `json:"authorize_url" dc:"用户需打开的 X 授权页"`
	State        string `json:"state" dc:"OAuth state（调试用；浏览器无需保存）"`
}

// OAuthCallbackReq X 授权回调（服务端换码后向 opener 回传 token）
type OAuthCallbackReq struct {
	g.Meta `path:"/x/oauth/callback" method:"get" tags:"X" summary:"X OAuth 回调换码"`
	Code   string `json:"code" in:"query" dc:"授权码"`
	State  string `json:"state" in:"query" dc:"state"`
	Error  string `json:"error" in:"query" dc:"OAuth 错误码"`
	Desc   string `json:"error_description" in:"query" dc:"OAuth 错误描述"`
}

// OAuthCallbackRes 回调本身返回 HTML；JSON 字段仅用于接口文档占位
type OAuthCallbackRes struct {
	OK bool `json:"ok"`
}
