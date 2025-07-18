module github.com/oomph-ac/oomph

go 1.24.1

require (
	github.com/akmalfairuz/legacy-version v1.5.4
	github.com/chewxy/math32 v1.10.1
	github.com/cooldogedev/spectrum v0.0.40-0.20250527034552-55ddfe1bba67
	github.com/df-mc/dragonfly v0.10.6-0.20250708145727-07da2e859609
	github.com/elliotchance/orderedmap/v2 v2.2.0
	github.com/ethaniccc/float32-cube v0.0.0-20250511224129-7af1f8c4ee12
	github.com/getsentry/sentry-go v0.27.0
	github.com/go-echarts/statsview v0.4.2
	github.com/go-gl/mathgl v1.2.0
	github.com/oomph-ac/multiversion v0.0.0-20250311011509-e9c78bda67c1
	github.com/oomph-ac/oconfig v0.0.0-20250315200330-e36f34d634e5
	github.com/sandertv/go-raknet v1.14.3-0.20250305181847-6af3e95113d6
	github.com/sandertv/gophertunnel v1.48.1
	github.com/zeebo/xxh3 v1.0.2
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cooldogedev/spectral v0.0.5 // indirect
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/df-mc/worldupgrader v1.0.19 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/go-echarts/go-echarts/v2 v2.5.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/pprof v0.0.0-20250630185457-6e76a2b096b5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hjson/hjson-go/v4 v4.4.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/onsi/ginkgo/v2 v2.23.4 // indirect
	github.com/quic-go/quic-go v0.52.0 // indirect
	github.com/rs/cors v1.11.0 // indirect
	github.com/samber/lo v1.49.1 // indirect
	github.com/scylladb/go-set v1.0.2 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.5.2 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/exp v0.0.0-20250711185948-6ae5c78190dc // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
)

//replace github.com/sandertv/go-raknet => github.com/tedacmc/tedac-raknet v0.0.4

//replace github.com/sandertv/gophertunnel => ../tedac-gophertunnel

replace github.com/df-mc/dragonfly => ../dragonfly

replace github.com/cooldogedev/spectrum => ../spectrum

replace github.com/oomph-ac/oconfig => ../oconfig

replace github.com/oomph-ac/multiversion => ../multiversion

replace github.com/akmalfairuz/legacy-version => ../legacy-version
