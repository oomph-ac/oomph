module github.com/oomph-ac/bedsim

go 1.25.0

require (
	github.com/df-mc/dragonfly v0.10.9
	github.com/go-gl/mathgl v1.2.0
	github.com/sandertv/gophertunnel v1.52.2
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/df-mc/worldupgrader v1.0.20 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/zaataylor/cartesian v0.0.0-20221028053253-3b3244d82727 // indirect
	golang.org/x/exp v0.0.0-20250103183323-7d7fa50e5329 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

replace (
	github.com/df-mc/dragonfly => ../../dragonfly
	github.com/sandertv/gophertunnel => ../../gophertunnel
)
