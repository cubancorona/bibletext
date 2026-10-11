package bibletext

import (
	"context"
	"errors"
)

type devDiagnosticRequestKey struct{}

func devDiagnosticRequestContext(ctx context.Context, id uint64) context.Context {
	if !devDiagnosticsEnabled {
		return ctx
	}
	return context.WithValue(ctx, devDiagnosticRequestKey{}, id)
}

func devDiagnosticRequestID(ctx context.Context) int64 {
	if !devDiagnosticsEnabled {
		return 0
	}
	id, _ := ctx.Value(devDiagnosticRequestKey{}).(uint64)
	return int64(id)
}

func devDiagnosticHTTPStatus(err error) int64 {
	if !devDiagnosticsEnabled {
		return 0
	}
	var httpErr *apiHTTPError
	if errors.As(err, &httpErr) {
		return int64(httpErr.StatusCode)
	}
	return 0
}

// Error classes deliberately omit error text, provider details, and URLs.
func devDiagnosticErrorClass(err error) int64 {
	if !devDiagnosticsEnabled || err == nil {
		return 0
	}
	switch {
	case errors.Is(err, context.Canceled):
		return 1
	case errors.Is(err, context.DeadlineExceeded):
		return 2
	case isNoKeyError(err):
		return 3
	default:
		return 4
	}
}

func devDiagnosticAction(action string) int64 {
	if !devDiagnosticsEnabled {
		return 0
	}
	switch action {
	case aiActionExplain:
		return 1
	case aiActionContext:
		return 2
	case aiActionTranslation:
		return 3
	case aiActionCompare:
		return 4
	case aiActionAsk:
		return 5
	default:
		return 0
	}
}
