package node

import (
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/expr-lang/expr"
)

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
