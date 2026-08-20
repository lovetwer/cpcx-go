package main

import (
	"context"
	"errors"
	"net/http"
)

type ctxKey string

const ctxUserKey ctxKey = "uid"

var errBadToken = errors.New("bad token")

func withUserID(r *http.Request, uid int64) context.Context {
	return context.WithValue(r.Context(), ctxUserKey, uid)
}
