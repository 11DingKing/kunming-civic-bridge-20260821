package service

import "fmt"

func abortBatchRow(index int, externalRef string, cause error) error {
	return fmt.Errorf("batch row %d (%s) failed: %w", index, externalRef, cause)
}
