package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/event"
	"github.com/oomph-ac/oomph/anticheat/oconfig"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/command"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var oomphHandler = newOomphHandler()

type OomphHandler struct {
	connected     map[string]*player.Player
	allowedAlerts map[string]*player.Player
	pMu           sync.RWMutex
}

func newOomphHandler() *OomphHandler {
	h := &OomphHandler{
		connected:     make(map[string]*player.Player),
		allowedAlerts: make(map[string]*player.Player),
	}
	// Example alert command w/ enable sub-command
	command.RegisterSubCommand("alerts", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(player.PermissionAlerts) {
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
		if !p.HasPerm(player.PermissionAlerts) {
			return nil
		}
		alertsEnumIdx := command.FindOrCreateEnum(pk, "oomph:alerts", []string{"alerts"})
		return &protocol.CommandOverload{
			Parameters: []protocol.CommandParameter{
				command.MakeEnumParam("alerts", alertsEnumIdx, false, false),
				command.MakeNormalParam("delayMs", protocol.CommandArgTypeInt, false),
			},
		}
	})
	// Example logs command
	command.RegisterSubCommand("logs", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(player.PermissionLogs) {
			return nil
		}
		logsEnumIdx := command.FindOrCreateEnum(pk, "oomph:logs", []string{"logs"})

		return &protocol.CommandOverload{
			Parameters: []protocol.CommandParameter{
				command.MakeEnumParam("logs", logsEnumIdx, false, false),
				command.MakeNormalParam("player", protocol.CommandArgTypeTarget, false),
			},
		}
	})

	// Example debug command
	command.RegisterSubCommand("debug", func(p command.Permissible, pk *packet.AvailableCommands) *protocol.CommandOverload {
		if !p.HasPerm(player.PermissionDebug) {
			return nil
		}
		debugEnumIdx := command.FindOrCreateEnum(pk, "oomph:debug", []string{"debug"})
		debugModes := append(player.DebugModeList, "type_message", "type_log")
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

func (h *OomphHandler) HandleJoin(ctx *event.Context[*player.Player]) {
	h.pMu.Lock()
	defer h.pMu.Unlock()

	p := ctx.Val()
	h.connected[p.Name()] = p
	if p.HasPerm(player.PermissionAlerts) {
		h.allowedAlerts[p.Name()] = p
	}
}

func (h *OomphHandler) HandleQuit(ctx *event.Context[*player.Player]) {
	h.pMu.Lock()
	defer h.pMu.Unlock()

	delete(h.connected, ctx.Val().Name())
	delete(h.allowedAlerts, ctx.Val().Name())
}

func (h *OomphHandler) HandleCommand(ctx *event.Context[*player.Player], command string, args []string) {
	p := ctx.Val()
	switch command {
	case "debug":
		if !p.HasPerm(player.PermissionDebug) {
			ctx.Cancel()
			return
		}

		if len(args) == 0 {
			p.Message("<red>Usage: /ac debug [debug_mode]</red>")
			return
		}
		if mode, ok := player.DebugModeMap[args[0]]; ok {
			p.Dbg.Toggle(mode)
			if p.Dbg.Enabled(mode) {
				p.Message("<green>Debug mode <yellow>%s</yellow> enabled</green>", args[0])
			} else {
				p.Message("<red>Debug mode <yellow>%s</yellow> disabled</red>", args[0])
			}
		} else if dbgType := args[0]; dbgType == "type_message" {
			p.Dbg.LoggingType = player.LoggingTypeMessage
			p.Message("<green>Debug type set to <yellow>message</yellow></green>")
		} else if dbgType == "type_log" {
			p.Dbg.LoggingType = player.LoggingTypeLogFile
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
		if !p.HasPerm(player.PermissionAlerts) {
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
		if !p.HasPerm(player.PermissionLogs) {
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

func (h *OomphHandler) HandlePunishment(ctx *event.Context[*player.Player], detection player.Detection, message *string) {

}

func (h *OomphHandler) HandleFlag(ctx *event.Context[*player.Player], dtc player.Detection, extraData []any) {
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
	h.broadcastAlert(alertMsg)
}

func (h *OomphHandler) broadcastAlert(alertMsg string) {
	h.pMu.RLock()
	recipients := make([]*player.Player, 0, len(h.allowedAlerts))
	for _, r := range h.allowedAlerts {
		recipients = append(recipients, r)
	}
	h.pMu.RUnlock()

	for _, r := range recipients {
		r.ReceiveAlert(alertMsg)
	}
}

func (h *OomphHandler) refreshAlertList() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()

	for range t.C {
		h.pMu.Lock()
		clear(h.allowedAlerts)
		for _, p := range h.connected {
			if p.HasPerm(player.PermissionAlerts) && p.ReceiveAlerts && time.Since(p.LastAlert) >= p.AlertDelay {
				h.allowedAlerts[p.Name()] = p
			}
		}
		h.pMu.Unlock()
	}
}
