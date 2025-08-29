package client

import (
	"context"
)

type accessTokenContextKey string

var accessTokenKey accessTokenContextKey = "access_token"

func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	ctx = context.WithValue(ctx, accessTokenKey, accessToken)
	return ctx
}

func GetAccessToken(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(accessTokenKey).(string)

	if !ok {
		return "", false
	}

	return v, true
}
