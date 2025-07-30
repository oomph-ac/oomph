package client

import (
	"time"
)

const (
	ByteToGBMultiplier float64 = 1.0 / (1024 * 1024 * 1024)
	GBToMBMultiplier   float64 = 1024.0
	GBToKBMultiplier   float64 = 1024.0 * 1024.0
)

var (
	gbIn      float64
	gbOut     float64
	gbProcIn  float64
	gbProcOut float64

	startedAt = time.Now()
)

func NetworkReport() networkReport {
	return networkReport{
		CompressionRatioIn:   1 - (gbIn / gbProcIn),
		CompressionRatioOut:  1 - (gbOut / gbProcOut),
		WrittenMB:            gbOut * GBToMBMultiplier,
		ReadMB:               gbIn * GBToMBMultiplier,
		AvgWriteRateKBPerSec: (gbOut * GBToKBMultiplier) / float64(time.Since(startedAt).Seconds()),
		AvgReadRateKBPerSec:  (gbIn * GBToKBMultiplier) / float64(time.Since(startedAt).Seconds()),
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
