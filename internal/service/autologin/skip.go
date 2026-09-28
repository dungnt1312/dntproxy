package autologin

import (
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

// classifyAccounts splits pasted accounts into ones needing a fresh login and
// ones already covered by a healthy connection (skip — re-importing healthy
// accounts just burns time and raises captcha risk on OpenAI's side).
func classifyAccounts(accounts []account, conns []domain.ProviderConnection, now time.Time) ([]account, []AccountResult) {
	var process []account
	var skipped []AccountResult
	for _, a := range accounts {
		conn, ok := healthyConnection(conns, a.Email, now)
		if !ok {
			process = append(process, a)
			continue
		}
		skipped = append(skipped, AccountResult{
			Email:        a.Email,
			Status:       "skipped",
			ConnectionID: conn.ID,
			Error: fmt.Sprintf("already healthy — token valid until %s",
				conn.ExpiresAt),
		})
	}
	return process, skipped
}

// healthyConnection returns a connection for the email that is active, has a
// refresh token, and whose access token outlives skipHealthHorizon.
func healthyConnection(conns []domain.ProviderConnection, email string, now time.Time) (*domain.ProviderConnection, bool) {
	for i := range conns {
		c := &conns[i]
		if !strings.EqualFold(strings.TrimSpace(c.Email), email) {
			continue
		}
		if !c.IsActive || c.RefreshToken == "" {
			continue
		}
		exp, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil || !exp.After(now.Add(skipHealthHorizon)) {
			continue
		}
		return c, true
	}
	return nil, false
}
