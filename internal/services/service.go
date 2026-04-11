package services

import "context"

// Service is implemented by all long-running background services.
type Service interface {
    Run(ctx context.Context) error
}