package player

func (p *Player) recoverError() {
	if v := recover(); v != nil && p.recoverFunc != nil {
		p.recoverFunc(p, v)
	}
}
