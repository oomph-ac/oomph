module github.com/oomph-ac/oomph

go 1.24.1

require (
	github.com/akmalfairuz/legacy-version v1.5.4
	github.com/chewxy/math32 v1.10.1
	github.com/cooldogedev/spectrum v0.0.40-0.20250527034552-55ddfe1bba67
	github.com/df-mc/dragonfly v0.10.9
	github.com/ethaniccc/float32-cube v0.0.0-20250511224129-7af1f8c4ee12
	github.com/getsentry/sentry-go v0.27.0
	github.com/go-echarts/statsview v0.4.2
	github.com/go-gl/mathgl v1.2.0
	github.com/oomph-ac/oconfig v0.0.0-20250315200330-e36f34d634e5
	github.com/sandertv/go-raknet v1.14.3-0.20250305181847-6af3e95113d6
	github.com/sandertv/gophertunnel v1.51.0
	github.com/zeebo/xxh3 v1.0.2
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cooldogedev/spectral v0.0.5 // indirect
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/df-mc/jsonc v1.0.5 // indirect
	github.com/df-mc/worldupgrader v1.0.20 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/go-echarts/go-echarts/v2 v2.5.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hjson/hjson-go/v4 v4.4.0 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/onsi/gomega v1.36.3 // indirect
	github.com/quic-go/quic-go v0.55.0 // indirect
	github.com/rs/cors v1.11.0 // indirect
	github.com/samber/lo v1.49.1 // indirect
	github.com/scylladb/go-set v1.0.2 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/oauth2 v0.32.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
)

//replace github.com/sandertv/go-raknet => github.com/tedacmc/tedac-raknet v0.0.4

//replace github.com/sandertv/gophertunnel => ../tedac-gophertunnel

replace github.com/sandertv/go-raknet => ../go-raknet

replace github.com/sandertv/gophertunnel => ../gophertunnel

replace github.com/df-mc/dragonfly => ../dragonfly

replace github.com/cooldogedev/spectrum => ../spectrum

replace github.com/oomph-ac/oconfig => ../oconfig

replace github.com/akmalfairuz/legacy-version => ../legacy-version
