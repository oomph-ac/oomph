package oconfig

const (
	ConfigVersion          uint64 = 6
	DefaultShutdownMessage        = "§cServer is restarting."
)

type Config struct {
	Version uint64 `json:"version" comment:"The version of the config file. This is used to ensure that the config file is compatible with the current version of Oomph.\nDO NOT MODIFY THIS VALUE."`

	Prefix             string `json:"prefix" comment:"The prefix to be used for Oomph."`
	CommandName        string `json:"command_name" comment:"The name of the command used access anti-cheat commands on the proxy. The default is 'ac' which will make the anti-cheat command '/ac'."`
	CommandDescription string `json:"command_description" comment:"The description of the command used access anti-cheat commands on the proxy."`

	GCPercent    int `json:"gc_percent" comment:"Golang's garbage collection percentage. If set to -1 (the default value), the proxy will only run garbage collection when reaching the memory soft limit.\nWe recommend NOT changing this value unless you know what you're doing."`
	MemThreshold int `json:"mem_threshold" comment:"A soft-limit for how much memory the Oomph proxy should use in megabytes. The default value is 1GB (1024MB).\nIf you are running Oomph on a container, we recommend setting this to roughly ~500MB lower to avoid OOM errors.\nIncrease this as neccessary to reduce garbage collection cycles."`

	LocalAddress  string `json:"local_addr" comment:"The address the proxy listens on for incoming connections. For the most part, just using a colon followed by the port is fine."`
	RemoteAddress string `json:"remote_addr" comment:"The address the proxy first connects to for the remote server."`
	BackupAddress string `json:"backup_addr" comment:"The address the proxy will connect to if the inital connection fails to the remote address."`

	ShutdownMessage string `json:"shutdown_message" comment:"The message players are disconnected with when the proxy is shut down and there is no available reconnect address."`
	ReconnectIP     string `json:"reconnect_ip" comment:"The IP address players connected to the proxy are transferred to in the event of a shutdown.\nIf this option is empty, players will be disconnected instead."`
	ReconnectPort   int    `json:"reconnect_port" comment:"The port players connected to the proxy are transferred to in the event of a shutdown.\nIf this option is empty, players will be disconnected instead."`

	UseLegacyEvents bool `json:"use_legacy_events" comment:"This option signifies wether the proxy should use the legacy event system to allow the remote server to handle punishments/flags.\nThis option is recommended to be set to false as the system will be removed in the future."`

	Resource ResourceOpts `json:"resource_opts" comment:"Options for your resource packs."`
	Network  NetworkOpts  `json:"network_opts" comment:"Options for configuring the network settings for Oomph."`
	Movement MovementOpts `json:"movement_opts" comment:"Options for configuring movement policies and strictness for Oomph."`
	Combat   CombatOpts   `json:"combat_opts" comment:"Options for configuring combat policies and strictness for Oomph."`

	Detections map[string]Detection `json:"detections" comment:"The configuration for each detection used by the proxy.\nThe allowed punishment types are:\n- none: No punishment will be applied to the player.\n- kick: The player will be kicked when they reach the maximum amount of violations allowed by the detection.\n- ban: The player will be banned when the maximum amount of violations is reached. A ban provider is required for this option to be applied.\nThe tags that can be applied in the flag message are:\n- {player}: The player's username.\n- {xuid}: The player's XBOX Live ID.\n- {violations}: The amount of violations that have been reached on the detection.\n- {prefix}: The prefix defined in the Oomph configuration."`
}

var (
	Global        Config
	DefaultConfig = Config{
		Version: ConfigVersion,

		Prefix: "§l§6o§eo§bm§ep§6h§7§r »",

		CommandName:        "ac",
		CommandDescription: "The command for anti-cheat functionality.",

		GCPercent:    -1,
		MemThreshold: 1024,

		LocalAddress:  ":19132",
		RemoteAddress: ":20000",

		ShutdownMessage: DefaultShutdownMessage,

		Resource: ResourceOpts{
			ResourceFolder: "resources/",
			RequirePacks:   true,
		},

		Network: NetworkOpts{
			AttemptFixChunks:     false,
			UpgradeChunksToBlobs: false,

			GlobalMovementCutoffThreshold: -1,
			MaxGhostBlockChain:            -1,
			MaxACKTimeout:                 60,
			MaxEntityRewind:               6,
			MaxKnockbackDelay:             10,
			MaxBlockUpdateDelay:           -1,
		},

		Movement: MovementOpts{
			CorrectionThreshold:         0.3,
			PersuasionThreshold:         0.002,
			AcceptClientPosition:        false,
			PositionAcceptanceThreshold: 0.09,
			AcceptClientVelocity:        false,
			VelocityAcceptanceThreshold: 0.03,
			LimitAllVelocity:            true,
			LimitAllVelocityThreshold:   10.0,
		},

		Combat: CombatOpts{
			LeftCPSLimit:  20,
			RightCPSLimit: 20,

			LeftCPSLimitMobile:  16,
			RightCPSLimitMobile: 15,

			MaximumAttackAngle:         85.0,
			EnableClientEntityTracking: true,
			AllowNonMobileTouch:        false,
			AllowSwitchInputMode:       false,
		},

		Detections: map[string]Detection{
			"Autoclicker_A": {
				MaxVl:      25.0,
				FlagMsg:    "{prefix} §e{player} §6is clicking too quickly §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
			},
			"Aim_A": {
				MaxVl:      5.0,
				FlagMsg:    "{prefix} §e{player} §6rotated suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
			},
			"BadPacket_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid packet",
				Punishment: PunishmentTypeBan,
			},
			"BadPacket_B": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6tried to attack themselves",
				Punishment: PunishmentTypeBan,
			},
			"BadPacket_C": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6tried to break blocks with an invalid packet",
				Punishment: PunishmentTypeBan,
			},
			"BadPacket_D": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6executed creative action in survival",
				Punishment: PunishmentTypeBan,
			},
			"BadPacket_E": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid movement",
				Punishment: PunishmentTypeBan,
			},
			"BadPacket_F": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid inventory action",
				Punishment: PunishmentTypeBan,
			},
			"EditionFaker_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6attempted to spoof their device information",
				Punishment: PunishmentTypeBan,
			},
			"EditionFaker_B": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6attempted to spoof their device information",
				Punishment: PunishmentTypeBan,
			},
			"EditionFaker_C": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6attempted to spoof their device information",
				Punishment: PunishmentTypeBan,
			},
			"Proxy_A": {
				MaxVl:      10.0,
				FlagMsg:    "{prefix} §e{player} §6is likely using a game proxy",
				Punishment: PunishmentTypeKick,
			},
			"Proxy_B": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6is connected with a game proxy",
				Punishment: PunishmentTypeBan,
			},
			"Hitbox_A": {
				MaxVl:      20.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Hitbox §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"InvMove_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6moving whilst in inventory",
				Punishment: PunishmentTypeBan,
			},
			"Killaura_A": {
				MaxVl:      5.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Killaura §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Nuker_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6tried to break blocks using an invalid packet §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Reach_A": {
				MaxVl:      10.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Reach §8(Raycast) §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Reach_B": {
				MaxVl:      30.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Reach §8(Raw) §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Scaffold_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent invalid action to place blocks §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Scaffold_B": {
				MaxVl:      25.0,
				FlagMsg:    "{prefix} §e{player} §6is placing with an invalid direction §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},

			// Cloud detections - max violations are ignored by default and is managed by the cloud instance itself.
			"Cloud_Scaffold": {
				FlagMsg:    "{prefix} §e{player} §6is building suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Cloud_Combat": {
				FlagMsg:    "{prefix} §e{player} §6is fighting suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Cloud_Aim": {
				FlagMsg:    "{prefix} §e{player} §6is aiming suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Cloud_Proxy": {
				FlagMsg:    "{prefix} §e{player} §6is using a game proxy §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
		},
	}
)
