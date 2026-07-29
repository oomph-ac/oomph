# Oomph example proxy

A standalone Minecraft Bedrock proxy with Oomph's anti-cheat pipeline already wired in. It accepts players on `local_addr`, connects each player to `remote_addr`, and falls back to `backup_addr` if the primary backend is unavailable.

## Requirements

- Go 1.26 or newer
- A Bedrock server that accepts the proxy's backend connections

PocketMine-MP servers must allow self-signed logins (`xbox-auth=false`). Apply the equivalent setting when another server implementation requires authenticated client chains.

## Setup

1. From the monorepo root, initialize submodules if needed: `git submodule update --init`.
2. Set `local_addr` and `remote_addr` in `oomph_config.hjson`.
3. Build and run the proxy:

   ```sh
   cd example-proxy
   go build -o example-proxy .
   ./example-proxy
   ```

   Use `-remote` to override `remote_addr` for one start:

   ```sh
   ./example-proxy -remote backend.example.com:19132
   ```

4. Connect Minecraft to `local_addr` instead of connecting directly to the backend server.

To grant Oomph commands to a moderator, add their exact player name to `moderators.list`, one name per line.

## Addresses and shutdown behavior

- `local_addr` is the public listener.
- `remote_addr` is the primary backend server.
- `-remote` overrides `remote_addr` for the current start.
- `backup_addr` is tried only when a new connection to the primary backend fails.
- `reconnect_ip` and `reconnect_port` are sent to connected players in a Transfer packet when this proxy shuts down.
- If `reconnect_ip` is empty or `reconnect_port` is invalid, players are disconnected with `shutdown_message` instead.

The fallback backend and shutdown transfer target are deliberately separate: the former keeps new logins available, while the latter moves already-connected players to another public listener.

## Included features

- Instant bidirectional packet forwarding through Oomph's native proxy integration
- Primary-to-backup backend fallback
- Graceful shutdown transfers
- Optional resource-pack loading and enforcement
- Per-player logs in `logs/`
- Moderator `/ac` commands for alerts, logs, and debugging
- Optional pprof endpoint

Set `PPROF_ENABLED=1` to enable pprof. It listens on `127.0.0.1:6060` by default; set `PPROF_ADDRESS` to override that address.

Detection alert templates support `{prefix}`, `{player}`, `{xuid}`, `{detection_type}`, `{detection_subtype}`, and `{violations}` placeholders.
