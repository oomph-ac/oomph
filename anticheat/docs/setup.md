# Setting up Oomph

Choose the setup that matches the software accepting players behind Oomph:

| Server software | Recommended integration |
| --- | --- |
| PocketMine-MP | [Standalone Oomph proxy with Oomph-PM](#pocketmine-mp) |
| Dragonfly | [Native Dragonfly listener](#dragonfly) |
| PowerNukkitX or another Bedrock server | [Standalone Oomph proxy](#other-server-software) |

Do not place both integrations in front of the same server. Dragonfly embeds
Oomph in the server process; the other paths run Oomph as the public-facing
RakNet listener.

## PocketMine-MP

Use Oomph's standalone proxy and install
[Oomph-PM](https://github.com/oomph-ac/oomph/anticheat-PM) on every PocketMine-MP
backend.

```text
Minecraft client -> Oomph :19132 -> PocketMine-MP 127.0.0.1:19133
                                      + Oomph-PM
```

### 1. Configure PocketMine-MP

Keep PocketMine-MP on a private address and let Oomph own the public Bedrock
port. For a proxy and backend on the same machine, the relevant
`server.properties` values are:

```properties
server-ip=127.0.0.1
server-port=19133
xbox-auth=off
```

Oomph authenticates the public client connection. The private backend must
accept Oomph's connection instead of trying to authenticate it with Xbox Live
again.

### 2. Install Oomph-PM

Build or download the current Oomph-PM plugin package, place the `.phar` in
PocketMine-MP's `plugins` directory, and start PocketMine-MP once to create its
configuration. In `plugin_data/Oomph/config.yml`, allow only the address from
which Oomph connects:

```yaml
Trusted-Proxy-Addresses:
  - "127.0.0.1"
  - "::1"
```

Use the proxy container or host address instead of loopback when the processes
run on different machines. Oomph-PM receives detection events and supplies the
PocketMine-side alerts, logs, punishments, and plugin events; it does not
replace PocketMine's network interface.

### 3. Start Oomph

Build the checked-in [`example/default`](../example/default) standalone
example:

```bash
cd example/default
go build -o oomph-proxy .
./oomph-proxy :19132 127.0.0.1:19133
```

Start PocketMine-MP before Oomph. Players join the Oomph address on port
`19132`, not the backend port.

## Dragonfly

Embed Oomph directly in Dragonfly. Do not run the standalone proxy in front of
the same Dragonfly listener.

```text
Minecraft client -> Dragonfly with Oomph :19132
```

Add Oomph and use the Dragonfly version selected by Oomph's
[`example/dragonfly/go.mod`](../example/dragonfly/go.mod). Go module `replace`
directives are not inherited from dependencies, so the application must carry
that Dragonfly replacement itself.

Configure the server listener like this:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/df-mc/dragonfly/server"
	oomphdragonfly "github.com/oomph-ac/oomph/anticheat/integration/dragonfly"
	"github.com/oomph-ac/oomph/anticheat/player"
)

func main() {
	ctx := context.Background()
	log := slog.Default()
	conf, err := server.DefaultConfig().Config(log)
	if err != nil {
		panic(err)
	}

	events := player.NewExampleEventHandler()
	conf.Listeners = []func(server.Config) (server.Listener, error){
		oomphdragonfly.Listener(ctx, oomphdragonfly.Config{
			Address: ":19132",
			Configure: func(p *player.Player) {
				p.HandleEvents(events)
			},
		}),
	}

	srv := conf.New()
	srv.CloseOnProgramEnd()
	srv.Listen()
	for range srv.Accept() {
	}
}
```

The listener uses Snappy compression by default unless the Dragonfly server
configuration explicitly selects another compression implementation. The
complete runnable version is in [`example/dragonfly`](../example/dragonfly).

## Other server software

Use Oomph's standalone proxy for Bedrock server software that does not have a
native integration. PowerNukkitX is shown here, but the same topology applies
to another backend that can accept a private, unauthenticated proxy
connection.

```text
Minecraft client -> Oomph :19132 -> PowerNukkitX 127.0.0.1:19133
```

### PowerNukkitX example

Set these values in PowerNukkitX's `server.properties`:

```properties
server-ip=127.0.0.1
server-port=19133
xbox-auth=off
```

Then build and start Oomph:

```bash
cd example/default
go build -o oomph-proxy .
./oomph-proxy :19132 127.0.0.1:19133
```

For containers or separate hosts, replace `127.0.0.1:19133` with the private
backend address reachable from Oomph. Keep `:19132` as the public listener (or
choose another public port) and restrict the backend UDP port to the Oomph host
at the firewall.

Backend-specific alerts or punishment hooks require an adapter comparable to
Oomph-PM. Packet validation and mitigation still run in the standalone proxy
without one.

## Verify the installation

After starting the backend and Oomph:

1. Ping the public Oomph address and confirm it reports the backend status.
2. Join through the public address and remain in the world for at least one
   minute.
3. Confirm the backend sees the player connection from the Oomph host.
4. Confirm the backend UDP port is not reachable from the public internet.
5. For PocketMine-MP, confirm Oomph-PM is enabled and trusts only the Oomph
   source address.

If the client reaches Oomph but the backend rejects login, first confirm that
the backend is private and that backend Xbox authentication is disabled. The
public Oomph listener should continue to authenticate players.
