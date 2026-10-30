// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package x

import (
	"context"

	"backend/api/x/v1"
)

type IXV1 interface {
	Post(ctx context.Context, req *v1.PostReq) (res *v1.PostRes, err error)
	OAuthStart(ctx context.Context, req *v1.OAuthStartReq) (res *v1.OAuthStartRes, err error)
	OAuthCallback(ctx context.Context, req *v1.OAuthCallbackReq) (res *v1.OAuthCallbackRes, err error)
	Verify(ctx context.Context, req *v1.VerifyReq) (res *v1.VerifyRes, err error)
}
