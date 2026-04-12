package custom

import (
	"fmt"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// NoOpQuotaChecker is a stub quota checker for providers without quota API support.
type NoOpQuotaChecker struct{}

// NewNoOpQuotaChecker creates a no-op quota checker.
func NewNoOpQuotaChecker() *NoOpQuotaChecker {
	return &NoOpQuotaChecker{}
}

// CheckQuota always returns nil (not supported).
func (n *NoOpQuotaChecker) CheckQuota(conn *domain.ProviderConnection) (*port.QuotaResult, error) {
	return nil, fmt.Errorf("quota check not supported for %s", conn.Provider)
}
