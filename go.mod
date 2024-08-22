module github.com/oomph-ac/oomph

go 1.22.1

toolchain go1.22.5

require (
	github.com/chewxy/math32 v1.10.1
	github.com/df-mc/dragonfly v0.9.18-0.20240818101738-29a214b79bd5
	github.com/disgoorg/json v1.1.0
	github.com/elliotchance/orderedmap/v2 v2.2.0
	github.com/ethaniccc/float32-cube v0.0.0-20230113135104-a65c4cb545c8
	github.com/getsentry/sentry-go v0.27.0
	github.com/go-gl/mathgl v1.1.0
	github.com/google/uuid v1.6.0
	github.com/quic-go/quic-go v0.45.2
	github.com/sandertv/go-raknet v1.14.1
	github.com/sandertv/gophertunnel v1.40.1
	github.com/sasha-s/go-deadlock v0.3.1
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/exp v0.0.0-20240808152545-0cdaa3abc0fa
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/df-mc/worldupgrader v1.0.16 // indirect
	github.com/gameparrot/goquery v0.2.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/petermattis/goid v0.0.0-20240327183114-c42a807a84ba // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/image v0.19.0 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/oauth2 v0.22.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
)

replace github.com/sandertv/gophertunnel v1.40.1 => ../gophertunnel

replace github.com/sandertv/go-raknet v1.14.1 => github.com/oomph-ac/go-raknet v0.0.1
