package player

const (
	PermissionAlerts uint64 = 1 << iota
	PermissionLogs
	PermissionDebug
)

func (p *Player) AddPerm(perm uint64) {
	p.perms = p.perms | perm
}

func (p *Player) RemovePerm(perm uint64) {
	p.perms = p.perms &^ perm
}

func (p *Player) HasPerm(perm uint64) bool {
	return p.perms&perm != 0
}
