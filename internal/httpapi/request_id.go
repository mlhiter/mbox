package httpapi

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Mbox-Request-ID"
const requestIDHeader = RequestIDHeader
const maxRequestIDRunes = 128

type requestIDContextKey struct{}

func withRequestID(ctx context.Context, value string) context.Context {
	requestID := sanitizeRequestID(value)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func sanitizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	count := 0
	for len(value) > 0 && count < maxRequestIDRunes {
		r, size := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError && size <= 1 {
			value = value[size:]
			continue
		}
		value = value[size:]
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			continue
		}
		builder.WriteRune(r)
		count++
	}
	return strings.TrimSpace(builder.String())
}
