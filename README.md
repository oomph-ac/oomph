# Oomph
Oomph is a military grade interception based anti-cheat proxy.

## How does oomph work?
Oomph is a middle-man between your server and any minecraft clients that want to join. Oomph processes all client packets
for the server and runs multiple checks on each one to detect for any possible cheats.

this allows for oomph to run on any server software, unlike traditional anti-cheats which are only compatible with a
specific one.

## Usage (Direct)
Oomph can be used in direct mode with dragonfly. This means it runs directly on a dragonfly server 
and connections are server -> client instead of server -> proxy -> client.
```go
config, err := readConfig(log)
if err != nil {
    panic(err)
}

// Anti-cheat start
if config.Oomph.Enabled {
    ac := oomph.New(log, ":19132")
    ac.Listen(&config, config.Name, []minecraft.Protocol{}, false)
    go func() {
        for {
            p, err := ac.Accept()
            if err != nil {
                return
            }
            p.SetCombatMode(2)
            p.SetMovementMode(2)
        }
    }()
}

srv := config.New()
srv.CloseOnProgramEnd()
srv.Listen()
```

## Usage (Proxy)
If you aren't using Dragonfly you'll have to use Oomph as a proxy.
```go
// 19132 is the port that players will connect to
ac := oomph.New(logger, ":19132")
// Accept oomph connections in another goroutine.
go func(){
    for {
        p, err := ac.Accept()
        if err != nil {
            return
        }
        p.Handle(newOomphHandler(p)) // The oomph handler can handle flags and punishments
    } 
}()

// 6969 is the port that the main server is running on, Oomph will redirect players to this address.
if err := ac.Start(":6969", config.Resources.Folder, []minecraft.Protocol{}, config.Resources.Required); err != nil {
    panic(err)
}
```

## Credits
Oomph is heavily influenced by [Esoteric](https://github.com/ethaniccc/Esoteric) and [Lumine](https://github.com/ethaniccc/Lumine).
Thanks, ethaniccc!

you're welcome Lolz thx for making base of oomph Tal -ethan
