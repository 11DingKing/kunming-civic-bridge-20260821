package service

import (
	"time"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func scheduleFeedbackRetry(receipt *domain.FeedbackReceipt, now time.Time, base time.Duration) {
	receipt.Status = domain.FeedbackDelivering
	receipt.LeaseUntil = nil
	receipt.NextAttemptAt = now.Add(base * time.Duration(1<<min(receipt.Attempt-1, 8)))
}
