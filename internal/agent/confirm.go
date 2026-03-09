package agent

import "encoding/json"

// ConfirmCallback is called before executing a non-read-only tool.
// Return true to approve execution, false to decline.
type ConfirmCallback func(toolName string, input json.RawMessage) (approved bool)

// IsReadOnly returns true for tools that only read data and don't modify anything.
func IsReadOnly(toolName string) bool {
	switch toolName {
	case "read", "grep", "find", "ls":
		return true
	default:
		return false
	}
}
