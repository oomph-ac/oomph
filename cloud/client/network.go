package client

import (
	"time"

	"go.uber.org/atomic"
)

const (
	ByteToGBMultiplier float64 = 1.0 / (1024 * 1024 * 1024)
	GBToMBMultiplier   float64 = 1024.0
	GBToKBMultiplier   float64 = 1024.0 * 1024.0
)

var (
	gbIn      atomic.Float64
	gbOut     atomic.Float64
	gbProcIn  atomic.Float64
	gbProcOut atomic.Float64

	startedAt = time.Now()
)

func NetworkReport() networkReport {
	in, out := gbIn.Load(), gbOut.Load()
	procIn, procOut := gbProcIn.Load(), gbProcOut.Load()
	return networkReport{
		CompressionRatioIn:   1 - (in / procIn),
		CompressionRatioOut:  1 - (out / procOut),
		WrittenMB:            out * GBToMBMultiplier,
		ReadMB:               in * GBToMBMultiplier,
		AvgWriteRateKBPerSec: (out * GBToKBMultiplier) / float64(time.Since(startedAt).Seconds()),
		AvgReadRateKBPerSec:  (in * GBToKBMultiplier) / float64(time.Since(startedAt).Seconds()),
		ElapsedTime:          time.Since(startedAt),
	}
}

type ConnectionInfo struct {
	FlushRate    time.Duration
	CmpThreshold uint32
}

type networkReport struct {
	CompressionRatioIn   float64
	CompressionRatioOut  float64
	WrittenMB            float64
	ReadMB               float64
	AvgWriteRateKBPerSec float64
	AvgReadRateKBPerSec  float64
	ElapsedTime          time.Duration
}
