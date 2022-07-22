package player

type Acknowledgements struct {
	AcknowledgeMap map[int64]func()
	HasTicked      bool

	awaitResTicks uint64
}

// Add adds an acknowledgement to run in the future to the map of acknowledgements.
func (a *Acknowledgements) Add(i int64, f func()) {
	a.AcknowledgeMap[i] = f
}

// Handle gets the acknowledgement in the map with the timestamp given in the function. If there is no acknowledgement
// found, then false is returned. If there is an acknowledgement, then it is removed from the map and the function is ran.
// "awaitResTicks" will also bet set to 0, as the client has responded to an acknowledgement.
func (a *Acknowledgements) Handle(i int64) bool {
	call, ok := a.AcknowledgeMap[i]
	if ok {
		a.awaitResTicks = 0
		a.Remove(i)
		call()
	}
	return ok
}

func (a *Acknowledgements) Remove(i int64) {
	delete(a.AcknowledgeMap, i)
}

// Validate checks if the client is still responding to acknowledgements sent to it. If it's determined that
// the client is not responding despite ticking, this function will return false. This is to prevent modified
// clients from breaking certain systems by simply ignoring acknowledgements we send.
func (a *Acknowledgements) Validate() bool {
	if !a.HasTicked {
		return true
	}

	a.HasTicked = false

	if len(a.AcknowledgeMap) == 0 {
		a.awaitResTicks = 0
		return true
	}

	a.awaitResTicks++
	return a.awaitResTicks < 200
}
