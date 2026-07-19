# Bedrock proxy

`proxy` is the standalone proxy core used by Oomph. It preserves a player's public connection while changing backend servers and provides identity forwarding, instant transfers, backend fallback, transfer-state cleanup, and stable player entity IDs without depending on Oomph's anti-cheat.

```go
p, err := proxy.Listen(ctx, proxy.Config{
	LocalAddress:  ":19132",
	RemoteAddress: "127.0.0.1:19133",
})
if err != nil {
	return err
}
defer p.Close()
return p.Serve(ctx)
```

Client authentication is enabled by default and should remain enabled on a public proxy listener. Disable authentication only on private backend servers that accept connections exclusively from the proxy.

Leave `NewHandler` unset for a proxy-only deployment. Set it to attach packet processing or backend lifecycle handling. Oomph exposes its ready-to-use integration through `github.com/oomph-ac/oomph/anticheat/integration/proxy`.
