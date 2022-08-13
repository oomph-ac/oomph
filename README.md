# Oomph
An interception based anti-cheat proxy.

## How does oomph work?
Oomph is a middle-man between your server and any minecraft clients that want to join. Oomph processes all client packets
for the server and runs multiple checks on each one to detect for any possible cheats.

this allows for oomph to run on any server software, unlike traditional anti-cheats which are only compatible with a
specific one.

## Usage (Direct)
Oomph can be used in direct mode with dragonfly. This means it runs directly on a dragonfly server 
and connections are server -> client instead of server -> proxy -> client.
```go
srv := server.New(&config.Config, logger)
srv.SetName("Velvet")
srv.CloseOnProgramEnd()
if err := srv.Start(); err != nil {
    logger.Fatalln(err)
}

// AntiCheat start
if config.Oomph.Enabled {
    go func() {
        ac := oomph.New(logger, config.Oomph.Address)
        if err := ac.Listen(srv, config.Server.Name, config.Resources.Required); err != nil {
            panic(err)
        }
        for {
            p, err := ac.Accept()
            if err != nil {
                return
            }
            p.Handle(newOomphHandler(p)) // Handle flags and punishments
        }
    }()
}
// AntiCheat end

for srv.Accept(nil) {
}
```

## Usage (Proxy)
If you aren't using Dragonfly you'll have to use Oomph as a proxy.
```go
go func() {
    // 19132 is the port that players will connect to
    ac := oomph.New(logger, ":19132")
    // 6969 is the port that the main server is running on, Oomph will redirect players to this address.
    if err := ac.Start(":6969"); err != nil {
        panic(err)
    }
    for {
        p, err := ac.Accept()
        if err != nil {
            return
        }
        p.Handle(newOomphHandler(p)) // Handle flags and punishments
    }
}()
```

## Credits
Oomph is heavily influenced by [esoteric](https://github.com/ethaniccc/Esoteric) and [lumine](https://github.com/ethaniccc/Lumine).
thank you, Ethaniccc for providing us with these!!!
