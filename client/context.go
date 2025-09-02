package client

import (
	"context"
	"errors"
)

var ErrAccessTokenNotFound = errors.New("access token could not be found within the provided context")
var ErrAuthInfoNotFound = errors.New("authorization info has not been provided throught the context")

type accessTokenContextKey string
type authContextKey string
type contextTypeKey string

type authInfo struct {
	accessToken string
	refreshToken string
	clientId string
	clientSecret string
}

type contextValue struct {
	ctxType string
	accessToken string
	authInfo authInfo
}

var accessTokenKey string = "access_token"
var authInfoKey string = "auth_info"
var ctxValueKey contextTypeKey = "context_val"


func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	val := contextValue{
		ctxType: accessTokenKey,
		accessToken: accessToken,
	}
	return context.WithValue(ctx, ctxValueKey, val)
}

func ContextWithClientInfo(ctx context.Context, accessToken string, refreshToken string, clientId string, clientSecret string) context.Context {
	info := authInfo{
		accessToken: accessToken,
		refreshToken: refreshToken,
		clientId: clientId,
		clientSecret: clientSecret,
	}

	val := contextValue{
		ctxType: authInfoKey,
		authInfo: info,
	}

	ctx = context.WithValue(ctx, ctxValueKey, val)

	return ctx
}

func GetClientInfoFromContext(ctx context.Context) (authInfo, error) {
	var zero authInfo

	v, ok := ctx.Value(ctxValueKey).(contextValue)

	if !ok {
		return zero, ErrAuthInfoNotFound
	}

	if v.ctxType != authInfoKey {
		return zero, ErrAuthInfoNotFound
	}

	return v.authInfo, nil
}

func GetAccessToken(ctx context.Context) (string, error) {
	v, ok := ctx.Value(ctxValueKey).(contextValue)

	if !ok {
		return "", ErrAccessTokenNotFound
	}

	if v.ctxType != accessTokenKey {
		return "", ErrAccessTokenNotFound
	}

	return v.accessToken, nil
}
