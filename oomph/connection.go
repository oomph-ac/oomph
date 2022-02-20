package oomph

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"net"
)

// Allower may be implemented to specifically allow or disallow players from joining a Server, by setting the specific
// Allower implementation through a call to Oomph.Allow.
type Allower interface {
	// Allow filters what connections are allowed to connect to Oomph. The address, identity data, and client data
	// of the connection are passed. If Admit returns false, the connection is closed with the string returned
	// as the disconnect message. WARNING: Use the client data at your own risk, it cannot be trusted because
	// it can be freely changed by the player connecting.
	Allow(addr net.Addr, d login.IdentityData, c login.ClientData) (string, bool)
}

// Closer may be implemented to handle when a player leaves the server. This is meant for cases where the player leaves
// before they join, where Dragonfly hasn't created the player yet and you can't use the quit handler.
type Closer interface {
	// Close allows you to handle the player leaving the server. This is necessary incase you're using Allower
	// and the player leaves before they fully join, it allows you to unset anything you may have saved such as a player session.
	Close(d login.IdentityData)
}
