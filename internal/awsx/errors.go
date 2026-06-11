package awsx

import "errors"

// Credential-state sentinels — the TUI inspects these to render the right message.
var (
	ErrCredentialsMissing = errors.New("credentials file is missing")
	ErrCredentialsExpired = errors.New("credentials are missing or expired")
)
