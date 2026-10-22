package service

import (
	"context"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/port"
)

// kiroEmailBackfillDelay lets the first token refresh run before the lookup.
const kiroEmailBackfillDelay = 15 * time.Second

// BackfillKiroEmailsOnce runs once at startup: Kiro connections created before
// dntproxy looked up account emails have no email and a placeholder name
// ("AWS IAM Identity Center Account 62"). It fills the email and replaces only
// placeholder names; names the operator chose are kept.
func BackfillKiroEmailsOnce(store port.CredentialStore, done <-chan struct{}) {
	select {
	case <-done:
		return
	case <-time.After(kiroEmailBackfillDelay):
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()
	if n := auth.BackfillKiroAccountEmails(ctx, store); n > 0 {
		log.Printf("[KIRO] filled account email for %d connection(s)", n)
	}
}
