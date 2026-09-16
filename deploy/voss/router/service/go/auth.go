package main

import (
	"crypto/subtle"
	"strings"
)

// OperatorAuth authenticates sovereign-operator API calls using a shared
// bearer token. The token is configured via the VOSS_OPERATOR_TOKEN
// environment variable; when unset the operator APIs fail closed.
type OperatorAuth struct {
	token string
}

func NewOperatorAuth(token string) *OperatorAuth {
	return &OperatorAuth{token: token}
}

// Authorized checks an Authorization header value like "Bearer <token>".
// Only the exact scheme prefix is accepted; anything else fails closed.
func (a *OperatorAuth) Authorized(header string) bool {
	if a.token == "" {
		return false
	}
	v, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(v), []byte(a.token)) != 1 {
		return false
	}
	return true
}