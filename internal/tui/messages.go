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
