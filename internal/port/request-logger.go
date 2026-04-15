package port

import "time"

// RequestLogger abstracts the lifecycle tracker for a single proxy request.
// Passed into adapters so they can enrich the log with detailed upstream info.
type RequestLogger interface {
	Upstream(url, method string, status int, duration time.Duration, err error)
	SetUsage(input, output int, source string)
	SetBodies(reqBody, respBody string)
}
