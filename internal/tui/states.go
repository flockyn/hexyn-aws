package tui

type state int

const (
	stateCheckingSession state = iota
	stateLogin
	stateSelectRegion
	stateMenu
	stateLoading
	stateSelectCluster
	stateSelectService
	stateSelectMethod
	stateInputs
	stateConfirmPut
	stateConfig
	stateExecuting
	stateResult
)
