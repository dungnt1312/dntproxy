package auth

import (
	"errors"

	"github.com/dungnt/dntproxy/internal/domain"
)

// ErrKind classifies Kiro auth failures the way kiro-shared's AuthError does.
type ErrKind string

const (
	KindInvalidAuth      ErrKind = "INVALID_AUTH"
	KindInvalidSSOAuth   ErrKind = "INVALID_SSO_AUTH"
	KindInvalidIdCAuth   ErrKind = "INVALID_IDC_AUTH"
	KindServerIssue      ErrKind = "SERVER_ISSUE"
	KindNetworkIssue     ErrKind = "NETWORK_ISSUE"
	KindUnexpected       ErrKind = "UNEXPECTED_ISSUE"
	KindCanceled         ErrKind = "CANCELED"
	KindAbandoned        ErrKind = "ABANDONED"
	KindInvalidUserInput ErrKind = "INVALID_USER_INPUT"
	KindSignInBlocked    ErrKind = "SIGN_IN_BLOCKED"
)

// KiroError is a classified auth error. Msg is the diagnostic text; UserMsg,
// when set, is the message safe to show an operator.
type KiroError struct {
	Kind    ErrKind
	Msg     string
	UserMsg string
	Err     error
}

// reauthHint is appended to credential-rejection errors so dntproxy's
// text-based revocation check (domain.IsAuthRevokedErrorText) sidelines the
// account instead of retrying it every minute.
const reauthHint = " (re-authenticate required)"

func (e *KiroError) Error() string {
	msg := e.Msg
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	switch e.Kind {
	case KindInvalidAuth, KindInvalidSSOAuth, KindInvalidIdCAuth:
		msg += reauthHint
	}
	return msg
}

func (e *KiroError) Unwrap() error { return e.Err }

func newKiroErr(kind ErrKind, msg string) *KiroError {
	return &KiroError{Kind: kind, Msg: msg}
}

func newKiroErrUser(kind ErrKind, msg, user string) *KiroError {
	return &KiroError{Kind: kind, Msg: msg, UserMsg: user}
}

func wrapKiroErr(kind ErrKind, msg string, cause error) *KiroError {
	return &KiroError{Kind: kind, Msg: msg, Err: cause}
}

// ErrorKind returns the Kiro error class, or "" for foreign errors.
func ErrorKind(err error) ErrKind {
	var ke *KiroError
	if errors.As(err, &ke) {
		return ke.Kind
	}
	return ""
}

// UserMessage returns the operator-facing text for an error, without the
// internal re-auth marker that Error() appends for revocation detection.
func UserMessage(err error) string {
	var ke *KiroError
	if errors.As(err, &ke) {
		if ke.UserMsg != "" {
			return ke.UserMsg
		}
		if ke.Err != nil {
			return ke.Msg + ": " + ke.Err.Error()
		}
		return ke.Msg
	}
	return err.Error()
}

// IsBadAuthIssue mirrors isBadAuthIssue: the credential itself is rejected and
// the account has to be re-authenticated.
func IsBadAuthIssue(err error) bool {
	switch ErrorKind(err) {
	case KindInvalidAuth, KindInvalidSSOAuth, KindInvalidIdCAuth:
		return true
	}
	return false
}

// IsRetryableAuthError reports transient failures (network / upstream 5xx)
// where retrying the same step can succeed.
func IsRetryableAuthError(err error) bool {
	switch ErrorKind(err) {
	case KindNetworkIssue, KindServerIssue:
		return true
	}
	return false
}

// IsDeadRefreshError reports a refresh failure that means the refresh token or
// client registration is unusable (needs re-auth), not a transient fault.
func IsDeadRefreshError(err error) bool {
	if err == nil {
		return false
	}
	return IsBadAuthIssue(err) || domain.IsAuthRevokedErrorText(err.Error())
}
