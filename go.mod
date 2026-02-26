module github.com/oomph-ac/oomph

go 1.25.1

require (
	github.com/chewxy/math32 v1.10.1
	github.com/df-mc/dragonfly v0.10.9
	github.com/ethaniccc/float32-cube v0.0.0-20250511224129-7af1f8c4ee12
	github.com/getsentry/sentry-go v0.40.0
	github.com/go-echarts/statsview v0.4.2
	github.com/go-gl/mathgl v1.2.0
	github.com/oomph-ac/oconfig v0.0.0-20250315200330-e36f34d634e5
	github.com/sandertv/go-raknet v1.15.1-0.20260112202637-beca0b10c217
	github.com/sandertv/gophertunnel v1.54.0
	github.com/zeebo/xxh3 v1.0.2
	golang.org/x/exp v0.0.0-20251209150349-8475f28825e9
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-oidc/v3 v3.17.0 // indirect
	github.com/df-mc/go-playfab v1.0.0 // indirect
	github.com/df-mc/go-xsapi v1.0.1 // indirect
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/df-mc/jsonc v1.0.5 // indirect
	github.com/df-mc/worldupgrader v1.0.20 // indirect
	github.com/go-echarts/go-echarts/v2 v2.5.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hjson/hjson-go/v4 v4.4.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/onsi/gomega v1.36.3 // indirect
	github.com/rs/cors v1.11.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

//replace github.com/sandertv/go-raknet => github.com/tedacmc/tedac-raknet v0.0.4

//replace github.com/sandertv/gophertunnel => ../tedac-gophertunnel

//replace github.com/sandertv/go-raknet => ../go-raknet

//replace github.com/sandertv/gophertunnel => ../gophertunnel

replace github.com/df-mc/dragonfly => ../dragonfly

replace github.com/oomph-ac/oconfig => ../oconfig
