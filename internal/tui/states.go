package tui

type state int

const (
	stateCheckingSession state = iota
	stateLogin
	stateSelectRegion
	stateMenu
	stateSelectEnv
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
