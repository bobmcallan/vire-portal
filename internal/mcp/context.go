package mcp

import "context"

// userContextKey is the context key for per-request user information.
type userContextKey struct{}

// UserContext holds per-request user identity for MCP proxy header injection.
type UserContext struct {
	UserID string
}

// WithUserContext returns a new context with the given UserContext attached.
func WithUserContext(ctx context.Context, uc UserContext) context.Context {
	return context.WithValue(ctx, userContextKey{}, uc)
}

// GetUserContext extracts the UserContext from the context, if present.
func GetUserContext(ctx context.Context) (UserContext, bool) {
	uc, ok := ctx.Value(userContextKey{}).(UserContext)
	return uc, ok
}
