package check

// init registers all completed checks.
func init() {
	RegisterCheck(NewAimAssistA())

	RegisterCheck(NewAutoClickerA())
	RegisterCheck(NewAutoClickerB())
	RegisterCheck(NewAutoClickerC())
	RegisterCheck(NewAutoClickerD())

	RegisterCheck(NewInvalidMovementC())

	RegisterCheck(NewKillAuraA())
	RegisterCheck(NewKillAuraB())

	RegisterCheck(NewOSSpoofer())

	RegisterCheck(NewReachA())

	RegisterCheck(NewTimerA())
}
