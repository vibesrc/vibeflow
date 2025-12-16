package node

import (
	"context"
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/expr-lang/expr"
)

// createContextAccessorMap creates a map with lowercase keys for expr-lang access.
// Supports: get(key), set(key, value), delete(key), keys()
func createContextAccessorMap(accessor ContextAccessor) map[string]any {
	return map[string]any{
		"get": func(key string) any {
			if accessor == nil {
				return nil
			}
			val, _ := accessor.Get(key)
			return val
		},
		"set": func(key string, value any) {
			if accessor != nil {
				accessor.Set(key, value)
			}
		},
		"delete": func(key string) {
			if accessor != nil {
				accessor.Delete(key)
			}
		},
		"keys": func() []string {
			if accessor == nil {
				return nil
			}
			return accessor.Keys()
		},
	}
}

// BuildExprEnv creates the standard environment for expr evaluation.
// Provides: msg (full message), payload (shorthand), metadata, context
func BuildExprEnv(msg *message.Message) map[string]any {
	return map[string]any{
		"msg": map[string]any{
			"id":       msg.ID,
			"payload":  msg.Payload,
			"metadata": msg.Metadata,
			"context":  msg.Context,
		},
		"payload":  msg.Payload,
		"metadata": msg.Metadata,
		"context":  msg.Context,
	}
}

// BuildExprEnvWithContext creates an environment with flow/node/global context accessors.
// Use this when you have access to the Go context in Process().
func BuildExprEnvWithContext(ctx context.Context, msg *message.Message) map[string]any {
	env := BuildExprEnv(msg)

	// Add context accessors if available (lowercase methods: get, set, delete, keys)
	env["flow"] = createContextAccessorMap(GetFlowContext(ctx))
	env["node"] = createContextAccessorMap(GetNodeContext(ctx))
	env["global"] = createContextAccessorMap(GetGlobalContext(ctx))

	return env
}

// EvalExpr evaluates an expression against a message and returns the result.
func EvalExpr(expression string, msg *message.Message) (any, error) {
	if expression == "" {
		return nil, nil
	}
	env := BuildExprEnv(msg)
	result, err := expr.Eval(expression, env)
	if err != nil {
		return nil, fmt.Errorf("expr eval error: %w", err)
	}
	return result, nil
}

// EvalExprWithContext evaluates an expression with access to flow/node/global state.
func EvalExprWithContext(ctx context.Context, expression string, msg *message.Message) (any, error) {
	if expression == "" {
		return nil, nil
	}
	env := BuildExprEnvWithContext(ctx, msg)
	result, err := expr.Eval(expression, env)
	if err != nil {
		return nil, fmt.Errorf("expr eval error: %w", err)
	}
	return result, nil
}

// EvalExprString evaluates an expression and returns the result as a string.
func EvalExprString(expression string, msg *message.Message) string {
	result, err := EvalExpr(expression, msg)
	if err != nil || result == nil {
		return ""
	}
	if s, ok := result.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", result)
}

// EvalExprStringWithContext evaluates an expression with context and returns the result as a string.
func EvalExprStringWithContext(ctx context.Context, expression string, msg *message.Message) string {
	result, err := EvalExprWithContext(ctx, expression, msg)
	if err != nil || result == nil {
		return ""
	}
	if s, ok := result.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", result)
}

// EvalExprBool evaluates an expression and returns the result as a bool.
func EvalExprBool(expression string, msg *message.Message) bool {
	result, err := EvalExpr(expression, msg)
	if err != nil || result == nil {
		return false
	}
	if b, ok := result.(bool); ok {
		return b
	}
	return false
}
