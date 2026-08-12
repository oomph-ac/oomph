package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/anticheat/integration/proxy"
	"github.com/oomph-ac/oomph/anticheat/oconfig"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var moderators map[string]struct{}

var remoteOverride = flag.String("remote", "", "override the remote address from the configuration")

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := sentry.Init(sentry.ClientOptions{Dsn: os.Getenv("SENTRY_DSN")}); err != nil {
		slog.Error("failed to initialize sentry", "error", err)
		os.Exit(1)
	}

	f, err := os.OpenFile("moderators.list", os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		slog.Error("failed to open moderators list", "error", err)
		os.Exit(1)
	}
	defer f.Close()
	moderators, err = loadModerators(f)
	if err != nil {
		slog.Error("failed to read moderators list", "error", err)
		os.Exit(1)
	}
}

func main() {
	flag.Parse()
	if err := oconfig.ParseJSON("oomph_config.hjson"); err != nil {
		if errors.Is(err, oconfig.ErrConfigCreated) || errors.Is(err, oconfig.ErrConfigUpdated) {
			slog.Info(err.Error())
			return
		}
		slog.Error("unable to parse config", "error", err)
		os.Exit(1)
	}
	remoteAddress := configuredRemoteAddress(oconfig.Global.RemoteAddress, *remoteOverride)
	if err := os.MkdirAll("logs", 0o755); err != nil {
		slog.Error("unable to create log directory", "error", err)
		return
	}
	startPprof()
	configureRuntime()

	status, err := minecraft.NewForeignStatusProvider(remoteAddress)
	if err != nil {
		slog.Error("unable to create status provider", "error", err)
		return
	}
	packs := loadResourcePacks()
	utils.InitializeBlockNameMapping()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := proxy.Listen(ctx, proxy.Config{
		LocalAddress:  oconfig.Global.LocalAddress,
		RemoteAddress: remoteAddress,
		Log:           slog.Default(),
		Listen: minecraft.ListenConfig{
			ResourcePacks:        packs,
			TexturePacksRequired: oconfig.Global.Resource.RequirePacks,
			StatusProvider:       status,
			FlushRate:            -1,
			AllowUnknownPackets:  true,
			AllowInvalidPackets:  true,
		},
		Dial:      dialBackends(oconfig.Global.BackupAddress, time.Minute),
		Configure: configurePlayer,
	})
	if err != nil {
		slog.Error("unable to start proxy", "error", err)
		return
	}
	defer p.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Serve(ctx) }()
	slog.Info("Oomph proxy is running", "address", oconfig.Global.LocalAddress, "remote", remoteAddress)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	select {
	case <-interrupt:
		shutdownPlayers()
		cancel()
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("proxy stopped", "error", err)
		}
	}
}

func startPprof() {
	if os.Getenv("PPROF_ENABLED") == "" {
		return
	}
	address := os.Getenv("PPROF_ADDRESS")
	if address == "" {
		address = "127.0.0.1:6060"
	}
	go func() {
		if err := http.ListenAndServe(address, nil); err != nil {
			slog.Error("pprof server stopped", "address", address, "error", err)
		}
	}()
}

func configureRuntime() {
	gcPercent := oconfig.Global.GCPercent
	if gcPercent < 100 && gcPercent != -1 {
		slog.Warn("GC percentage below 100 may cause high CPU usage; using 100", "configured", gcPercent)
		gcPercent = 100
	}
	debug.SetGCPercent(gcPercent)
	debug.SetMemoryLimit(int64(oconfig.Global.MemThreshold) * 1024 * 1024)
}

func loadResourcePacks() []*resource.Pack {
	if oconfig.Global.Resource.ResourceFolder == "" {
		return nil
	}
	packs, err := utils.ResourcePacks(oconfig.Global.Resource.ResourceFolder, "content_keys.json")
	if err != nil {
		slog.Error("unable to load resource packs", "error", err)
		return nil
	}
	return packs
}

func configurePlayer(p *player.Player) {
	if _, ok := moderators[p.Name()]; ok {
		p.AddPerm(player.PermissionAlerts)
		p.AddPerm(player.PermissionLogs)
		p.AddPerm(player.PermissionDebug)
	}
	f, err := os.OpenFile(filepath.Join("logs", p.Name()+".log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		slog.Error("failed to create player log", "player", p.Name(), "error", err)
		p.Disconnect("Failed to create player log.")
		return
	}
	p.SetLog(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})))
	p.SetCloser(func() { _ = f.Close() })
	p.SetRecoverFunc(recoverPlayerFn)
	p.HandleEvents(oomphHandler)
}

func dialBackends(backup string, timeout time.Duration) proxy.DialFunc {
	return func(ctx context.Context, primary string, identity login.IdentityData, client login.ClientData, _ string) (proxy.Backend, error) {
		var dialErrors []error
		for _, address := range backendAddresses(primary, backup) {
			dialCtx, cancel := context.WithTimeout(ctx, timeout)
			conn, err := (minecraft.Dialer{
				IdentityData:        identity,
				ClientData:          client,
				KeepXBLIdentityData: true,
				FlushRate:           -1,
				EnableBatchReading:  true,
			}).DialContext(dialCtx, "raknet", address)
			cancel()
			if err == nil {
				return conn, nil
			}
			dialErrors = append(dialErrors, fmt.Errorf("%s: %w", address, err))
		}
		return nil, fmt.Errorf("unable to connect to a backend: %w", errors.Join(dialErrors...))
	}
}

func shutdownPlayers() {
	address, port, transfer := shutdownTarget(oconfig.Global.ReconnectIP, oconfig.Global.ReconnectPort)
	oomphHandler.pMu.RLock()
	players := make([]*player.Player, 0, len(oomphHandler.connected))
	for _, p := range oomphHandler.connected {
		players = append(players, p)
	}
	oomphHandler.pMu.RUnlock()

	for _, p := range players {
		if !transfer {
			reason := oconfig.Global.ShutdownMessage
			if err := p.SendPacketToClient(&packet.Disconnect{Message: reason, FilteredMessage: reason}); err != nil {
				slog.Warn("failed to send shutdown disconnect", "player", p.Name(), "error", err)
			} else if err := p.Flush(); err != nil {
				slog.Warn("failed to flush shutdown disconnect", "player", p.Name(), "error", err)
			}
			_ = p.Close()
			continue
		}
		if err := p.SendPacketToClient(&packet.Transfer{Address: address, Port: port}); err != nil {
			slog.Warn("failed to send shutdown transfer", "player", p.Name(), "error", err)
			continue
		}
		if err := p.Flush(); err != nil {
			slog.Warn("failed to flush shutdown transfer", "player", p.Name(), "error", err)
		}
	}
}
