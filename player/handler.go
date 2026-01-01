package player

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/event"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph/player/command"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/maps"
)

type EventHandler interface {
	// HandleJoin is called when a player connects to the proxy.
	HandleJoin(ctx *event.Context[*Player])
	// HandleQuit is called when a player disconnects from the proxy.
	HandleQuit(ctx *event.Context[*Player])

	// HandleCommand is called when a player sends an oomph command.
	HandleCommand(ctx *event.Context[*Player], command string, args []string)

	// HandlePunishment is called when a detection triggers a punishment for a player.
	HandlePunishment(ctx *event.Context[*Player], detection Detection, message *string)
	// HandleFlag is called when a detection flags a player.
	// extraData is a list of key-value pairs in slog style: key1, val1, key2, val2, ... and can be converted
	// to a map with utils.KeyValsToMap
	HandleFlag(ctx *event.Context[*Player], detection Detection, extraData []any)
}

// NopEventHandler is an event handler that does nothing.
type NopEventHandler struct{}

func (NopEventHandler) HandleJoin(*event.Context[*Player])                           {}
func (NopEventHandler) HandleQuit(*event.Context[*Player])                           {}
func (NopEventHandler) HandleCommand(*event.Context[*Player], string, []string)      {}
func (NopEventHandler) HandlePunishment(*event.Context[*Player], Detection, *string) {}
func (NopEventHandler) HandleFlag(*event.Context[*Player], Detection, []any)         {}

type ExampleEventHandler struct {
	connected     map[string]*Player
	allowedAlerts map[string]*Player
	pMu           sync.RWMutex
}

func NewExampleEventHandler() *ExampleEventHandler {
	h := &ExampleEventHandler{
		connected:     make(map[string]*Player),
		allowedAlerts: make(map[string]*Player),
	}
	// Example alert command w/ enable sub-command
	command.RegisterSubCommand("alerts", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(PermissionAlerts) {
			return nil
		}
		alertsEnumIdx := command.FindOrCreateEnum(pk, "oomph:alerts", []string{"alerts"})
		enableEnumIdx := command.FindOrCreateEnum(pk, "enabled", []string{"true", "false", "enable", "disable"})

		return &protocol.CommandOverload{
			Parameters: []protocol.CommandParameter{
				command.MakeEnumParam("alerts", alertsEnumIdx, false, false),
				{
					Name:     "enable_alerts",
					Type:     protocol.CommandArgValid | protocol.CommandArgEnum | enableEnumIdx,
					Optional: false,
				},
			},
		}
	})
	// Example alert command w/ delay sub-command
	command.RegisterSubCommand("alerts", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(PermissionAlerts) {
			return nil
		}
		alertsEnumIdx := command.FindOrCreateEnum(pk, "oomph:alerts", []string{"alerts"})
		return &protocol.CommandOverload{
			Parameters: []protocol.CommandParameter{
				command.MakeEnumParam("alerts", alertsEnumIdx, false, false),
				mkNormalParam("delayMs", protocol.CommandArgTypeInt, false),
			},
		}
	})
	// Example logs command
	command.RegisterSubCommand("logs", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(PermissionLogs) {
			return nil
		}
		logsEnumIdx := command.FindOrCreateEnum(pk, "oomph:logs", []string{"logs"})

		return &protocol.CommandOverload{
			Parameters: []protocol.CommandParameter{
				command.MakeEnumParam("logs", logsEnumIdx, false, false),
				mkNormalParam("player", protocol.CommandArgTypeTarget, false),
			},
		}
	})

	// Example debug command
	command.RegisterSubCommand("debug", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(PermissionDebug) {
			return nil
		}
		debugEnumIdx := command.FindOrCreateEnum(pk, "oomph:debug", []string{"debug"})
		debugModes := append(DebugModeList, "type_message", "type_log")
		debugModesEnumIdx := command.FindOrCreateDynamicEnum(pk, "oomph:debug_modes", debugModes)

		return &protocol.CommandOverload{
			Parameters: []protocol.CommandParameter{
				command.MakeEnumParam("debug", debugEnumIdx, false, false),
				command.MakeEnumParam("mode", debugModesEnumIdx, true, true),
			},
		}
	})
	go h.refreshAlertList()
	return h
}

func (h *ExampleEventHandler) HandleJoin(ctx *event.Context[*Player]) {
	h.pMu.Lock()
	defer h.pMu.Unlock()

	p := ctx.Val()
	h.connected[p.Name()] = p
	if p.HasPerm(PermissionAlerts) {
		h.allowedAlerts[p.Name()] = p
	}
}

func (h *ExampleEventHandler) HandleQuit(ctx *event.Context[*Player]) {
	h.pMu.Lock()
	defer h.pMu.Unlock()

	delete(h.connected, ctx.Val().Name())
	delete(h.allowedAlerts, ctx.Val().Name())
}

func (h *ExampleEventHandler) HandleCommand(ctx *event.Context[*Player], command string, args []string) {
	p := ctx.Val()
	switch command {
	case "debug":
		if !p.HasPerm(PermissionDebug) {
			ctx.Cancel()
			return
		}

		if len(args) == 0 {
			p.Message("<red>Usage: /ac debug [debug_mode]</red>")
			return
		}
		if mode, ok := DebugModeMap[args[0]]; ok {
			p.Dbg.Toggle(mode)
			if p.Dbg.Enabled(mode) {
				p.Message("<green>Debug mode <yellow>%s</yellow> enabled</green>", args[0])
			} else {
				p.Message("<red>Debug mode <yellow>%s</yellow> disabled</red>", args[0])
			}
		} else if dbgType := args[0]; dbgType == "type_message" {
			p.Dbg.LoggingType = LoggingTypeMessage
			p.Message("<green>Debug type set to <yellow>message</yellow></green>")
		} else if dbgType == "type_log" {
			p.Dbg.LoggingType = LoggingTypeLogFile
			p.Message("<green>Debug type set to <yellow>log file</yellow></green>")
		} else if dbgType == "gmc" && len(os.Getenv("OOMPH_GAMEMODE_TEST_BECAUSE_DEV")) > 0 {
			p.SendPacketToClient(&packet.SetPlayerGameType{
				GameType: packet.GameTypeCreative,
			})
			p.Message("<green>Set client-side gamemode to <yellow>creative</yellow></green>")
		} else {
			p.Message("<red>Invalid debug mode</red>")
		}
	case "alerts":
		if !p.HasPerm(PermissionAlerts) {
			ctx.Cancel()
			return
		}
		if len(args) == 0 {
			p.Message("<red>Usage: /ac alerts [enable]</red>")
			return
		}

		if args[0] == "true" || args[0] == "enable" {
			p.ReceiveAlerts = true
			p.Message("<green>Alerts enabled</green>")
		} else if args[0] == "false" || args[0] == "disable" {
			p.ReceiveAlerts = false
			p.Message("<red>Alerts disabled</red>")
		} else if msDelay, err := strconv.Atoi(args[0]); err == nil {
			p.AlertDelay = time.Duration(msDelay) * time.Millisecond
			p.Message("<green>Alert delay set to <yellow>%s</yellow></green>", p.AlertDelay)
		} else {
			p.Message("<red>Usage: /ac alerts [true:false]</red>")
		}
	case "logs":
		if !p.HasPerm(PermissionLogs) {
			ctx.Cancel()
			return
		}
		if len(args) == 0 {
			p.Message("<red>Usage: /ac logs [player]</red>")
			return
		}
		targetPlayer := args[0]
		target, ok := h.connected[targetPlayer]
		if !ok {
			p.Message("<red>Player not found</red>")
			return
		}
		p.Message("<green>Logs for</green> <yellow>%s</yellow>", targetPlayer)
		for _, dtc := range target.Detections() {
			if dtc.Metadata().Violations < 0.1 {
				continue
			}
			p.Message("<yellow>%s</yellow> <grey>(</grey><red>%s</red><grey>)</grey> <grey>[</grey><red>x%.2f</red><grey>]</grey>", dtc.Type(), dtc.SubType(), dtc.Metadata().Violations)
		}
	}
}

func (h *ExampleEventHandler) HandlePunishment(ctx *event.Context[*Player], detection Detection, message *string) {

}

func (h *ExampleEventHandler) HandleFlag(ctx *event.Context[*Player], dtc Detection, extraData []any) {
	p := ctx.Val()
	m := dtc.Metadata()

	// Fast key build and value formatting.
	dtcKey := dtc.Type() + "_" + dtc.SubType()
	msgTmpl := oconfig.DtcOpts(dtcKey).FlagMsg
	viol := strconv.FormatFloat(m.Violations, 'f', 2, 64)

	// One-pass replace.
	alertMsg := strings.NewReplacer(
		"{prefix}", oconfig.Global.Prefix,
		"{player}", p.IdentityDat.DisplayName,
		"{xuid}", p.IdentityDat.XUID,
		"{detection_type}", dtc.Type(),
		"{detection_subtype}", dtc.SubType(),
		"{violations}", viol,
	).Replace(msgTmpl)

	h.pMu.RLock()
	recipients := make([]*Player, 0, len(h.allowedAlerts))
	for _, r := range h.allowedAlerts {
		recipients = append(recipients, r)
	}
	h.pMu.RUnlock()

	for _, r := range recipients {
		r.ReceiveAlert(alertMsg)
	}
}

func (h *ExampleEventHandler) refreshAlertList() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()

	for range t.C {
		h.pMu.Lock()
		maps.Clear(h.allowedAlerts)
		for _, p := range h.connected {
			if p.HasPerm(PermissionAlerts) && p.ReceiveAlerts && time.Since(p.LastAlert) >= p.AlertDelay {
				h.allowedAlerts[p.Name()] = p
			}
		}
		h.pMu.Unlock()
	}
}
