package tui

import (
	"hexyn-aws/internal/awsx"

	"github.com/charmbracelet/bubbles/list"
)

type resultMsg struct {
	message string
	err     error
}

type listMsg struct {
	items []list.Item
	err   error
	next  state
}

type sessionMsg struct {
	session awsx.Session
	err     error
}

// previewMsg carries the parsed parameters of a PUT's input file, gathered before
// upload so the confirmation screen can display them.
type previewMsg struct {
	params []awsx.Parameter
	err    error
}
