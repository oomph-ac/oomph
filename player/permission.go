package player

const (
	PermissionAlerts Permissions = 1 << iota
	PermissionLogs
	PermissionDebug
)

type Permissions uint16

func (p *Player) AddPerm(perm Permissions) {
	p.perms = p.perms | perm
}

func (p *Player) RemovePerm(perm Permissions) {
	p.perms = p.perms &^ perm
}

func (p *Player) HasPerm(perm Permissions) bool {
	return p.perms&perm != 0
}
