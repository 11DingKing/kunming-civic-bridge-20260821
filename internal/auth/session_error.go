package auth

import "fmt"

func describeSessionFailure(kind error, sessionID string) error {
	return fmt.Errorf("session %s cannot be resolved: %v", sessionID, kind)
}
