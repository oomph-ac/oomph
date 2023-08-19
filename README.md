# Oomph
Oomph is a military grade interception based anti-cheat proxy for Minecraft: Bedrock Edition.

## How does oomph work?
Oomph acts as an intermediary between your server and Minecraft clients, processing all client packets. It performs various checks on all packets to detect potential cheats. This versatility lets Oomph function on different server software.

Unlike other anti-cheats, Oomph emulates player actions and sends the calculated outcome to the server. If the client's outcome differs from the server's, corrections are applied to ensure fair gameplay, even with varying latencies. For instance, if a player uses fly cheats, Oomph verifies and sends legitimate movement instead of the cheating player's movement. This same concept applies to combat.

## Usage (Dragonfly)
Oomph can be used with Dragonfly without the overhead of a proxy.
```go
config, err := readConfig(log)
if err != nil {
    panic(err)
}

ac := oomph.New(log, ":19132")
ac.Listen(&config, config.Name, []minecraft.Protocol{}, false, false)

srv := config.New()
srv.CloseOnProgramEnd()

go func() {
	for {
		p, err := ac.Accept()
		if err != nil {
			return
		}

		p.UsePacketBuffering(false)
		p.SetCombatMode(2)
		p.SetMovementMode(2)
		p.MovementInfo().SetAcceptablePositionOffset(0.3)

		p.SetCombatCutoff(3)
		p.SetKnockbackCutoff(3)

		p.Handle(newOomphHandler(p))

		f, err := os.OpenFile("./logs/"+p.Conn().ClientData().ThirdPartyName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
			return
		}
		p.Log().SetOutput(f)
	}
}()

srv.Listen()

for srv.Accept(nil) {}
```

## Usage (Proxy)
```go
// Oomph will run and accept connections on the port 19132.
ac := oomph.New(log, ":19132")
go func() {
	for {
		p, err := ac.Accept()
		if err != nil {
			panic(err)
		}

		p.UsePacketBuffering(false)
		p.SetCombatMode(2)
		p.SetMovementMode(2)
		p.MovementInfo().SetAcceptablePositionOffset(0.3)

		p.SetCombatCutoff(3)
		p.SetKnockbackCutoff(3)

		p.Handle(newOomphHandler(p))

		f, err := os.OpenFile("./logs/"+p.Conn().ClientData().ThirdPartyName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
			return
		}
		p.Log().SetOutput(f)
	}
}()

// Oomph will re-direct connections to the server running on port 20000.
ac.Start(":20000", ".", []minecraft.Protocol{}, false, false)
```

## Configuration
You may configure settings for your players are you may like. Here are a few options you are able to change and what they do:
- **Packet Buffering** [ p.UsePacketBuffering(bool) ]
    Packet Buffering keeps a buffer of player packets before processing, this contributes to helping players with unstable connections seem as if they are not having issues at all. If a player is running below 20-TPS, packet buffering will fill with their last processed input to make movement run at 20 ticks-per-second on the server side. This type of functionality can be found in games such as VALORANT and Overwatch. **However, this feature is experimental, and it is not advised to run it on production.**
- **Combat Mode** [ p.SetCombatMode(AuthorityType) ]
    Setting the combat mode will change how Oomph will validate combat actions. The following supported modes are listed below:
    - ModeClientAuthoritative (0) -> Oomph will not validate any combat action(s), and send to the server what the client deems legitimate.
    - ModeSemiAuthoritative (1) -> Oomph with it's latency compensation will estimate an exact (ticked) position of where entities are on the client side, and use a reach check to determine wether or not the player is using cheats. Hits will not be cancelled, but alerts will be triggered when a player does not have legitimate reach.
    - ModeFullAuthoritative (2) -> Oomph will take full control over combat, and will make a list of positions the entity has been in the past. When a player sends an entity attack, Oomph rewinds to the client's tick and determines wether or not the player will be able to hit the entity. When a player sends a missed swing, we find entities within 4.5 blocks (search range) and determine wether or not the player would actually be able to hit an entity (this would be a "client mis-prediction".) This mode does not flag any check, and instead will only send attacks it deems legitimate.
- **Movement Mode** [ p.SetMovementMode(AuthorityType) ]
    Setting the movement mode will change how Oomph will validate movement. The following supported modes are listed below:
    - ModeClientAuthoritative (0) -> Oomph will not validate any movement, and send the server the client's movement.
    - ModeSemiAuthoritative (**TO BE REVISED**) (1) -> Oomph will recieve movement and deem wether or not it is valid or not. If enough illegitimate movements are made in a row, Oomph will flag the player in a movement check. Then, Oomph will set it's calculated movement to the client.
    - ModeFullAuthoritative (2) -> In this mode, Oomph derives its own calculated position from the client's inputs. If it detects a notable divergence (adjustable), it sends a correction. When the client receives the correction, their position will be in sync with the Oomph's position. In this mode, Oomph solely transmits its calculated position to the server, omitting any client movement. As a result, movement cheats become ineffective since other players on the server cannot detect or be influenced by such cheats.
- **Acceptable Position Offset** [ p.SetAcceptablePositionOffset(float32) ]
    *This setting is to only be used when the movement mode is set to ModeFullAuthoritative.* This determines how much difference there can be between Oomph's and the player's positions before a correction needs to be sent.
- **Combat Cutoff** [ p.SetCombatCutoff(int) ]
    *This setting is to only be used when the combat mode is set to ModeFullAuthoritative.* This determines the maximum allowed number of ticks allowed for combat rewind. Assuming the combat cutoff is set to the default 6 ticks (300ms), when a player with a higher latency (e.g 350ms/7 ticks) attemps to attack an entity, Oomph will rewind the position only 300 milliseconds into the past and validate the hit with that position. 
- **Knockback Cutoff** [ p.SetKnockbackCutoff(int) ]
    *This setting is to only be used when the movement mode is set to ModeFullAuthoritative.* This determines the amount of ticks (50ms per tick) in latency the player is allowed before using server-knockback (setting the player's knockback instantly). This will mitigate the advantage of higher latency players having delayed knockback, at the expense of how smooth their experience on the server may be. Please check out [this](https://bugs.mojang.com/browse/BDS-18632) issue on the MC:BE issue tracker, and vote to get non-smoothed movement corrections resolved.

## Credits
* [ethaniccc](https://www.github.com/ethaniccc) - Created the systems for validating combat and movement, while keeping lag-compensation in mind.
* [JustTalDevelops](https://github.com/JustTalDevelops) - Created the base of Oomph, making it able to intercept packets, and avoiding pesky import cycles.
