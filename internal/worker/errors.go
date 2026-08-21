package worker

import "fmt"

func reevalFailure(operation string, cause error) error {
	return fmt.Errorf("%s failed: %v (classification: %v)", operation, cause, ErrRuleReeval)
}
