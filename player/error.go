package player

func (p *Player) recoverError() {
	if p.recoverFunc == nil {
		return
	}
	if v := recover(); v != nil {
		p.recoverFunc(p, v)
	}
}
