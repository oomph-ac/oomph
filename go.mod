module github.com/oomph-ac/oomph

go 1.21.1

toolchain go1.21.2

require (
	github.com/chewxy/math32 v1.10.1
	github.com/df-mc/dragonfly v0.9.13-0.20240209111632-1cb1df2a7b7a
	github.com/elliotchance/orderedmap/v2 v2.2.0
	github.com/ethaniccc/float32-cube v0.0.0-20230113135104-a65c4cb545c8
	github.com/getsentry/sentry-go v0.27.0
	github.com/go-echarts/statsview v0.3.4
	github.com/go-gl/mathgl v1.1.0
	github.com/oomph-ac/mv v0.0.0-20240210123432-1cb78e0dd37d
	github.com/sandertv/gophertunnel v1.35.0
	github.com/sasha-s/go-deadlock v0.3.1
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/exp v0.0.0-20240205201215-2c58cdc269a3
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/df-mc/worldupgrader v1.0.12 // indirect
	github.com/disgoorg/disgo v0.17.2 // indirect
	github.com/disgoorg/json v1.1.0 // indirect
	github.com/disgoorg/snowflake/v2 v2.0.1 // indirect
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/go-echarts/go-echarts/v2 v2.3.3 // indirect
	github.com/go-jose/go-jose/v3 v3.0.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/pprof v0.0.0-20211214055906-6f57359322fd // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/petermattis/goid v0.0.0-20231207134359-e60b3f734c67 // indirect
	github.com/pkg/profile v1.7.0 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/rs/cors v1.10.1 // indirect
	github.com/sandertv/go-raknet v1.12.1 // indirect
	github.com/sasha-s/go-csync v0.0.0-20240107134140-fcbab37b09ad // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.19.0 // indirect
	golang.org/x/image v0.15.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/oauth2 v0.17.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
)

replace github.com/sandertv/gophertunnel v1.35.0 => github.com/oomph-ac/gophertunnel v0.0.0-20240210012800-073ea43d2569

replace github.com/df-mc/dragonfly v0.9.13-0.20240209111632-1cb1df2a7b7a => github.com/oomph-ac/dragonfly v0.0.0-20240207223737-96dfd1fafe9e

replace github.com/sandertv/go-raknet v1.12.1 => github.com/tedacmc/tedac-raknet v0.0.4
