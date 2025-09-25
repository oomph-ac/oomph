package client

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"
)

const (
	ByteToGBMultiplier float64 = 1.0 / (1024 * 1024 * 1024)
	ByteToMBMultiplier float64 = 1.0 / (1024 * 1024)
	ByteToKBMultiplier float64 = 1.0 / 1024
)

// GlobalStats holds global network statistics across all clients
var GlobalStats = &NetworkStats{}

// NetworkStats provides comprehensive network statistics tracking
type NetworkStats struct {
	mu sync.RWMutex

	// Byte counters (in bytes)
	bytesRead     atomic.Uint64 // Total bytes read from network
	bytesWritten  atomic.Uint64 // Total bytes written to network
	origBytesRead atomic.Uint64 // Original uncompressed bytes received
	origBytesSent atomic.Uint64 // Original uncompressed bytes to send

	// Packet counters
	packetsRead atomic.Uint64
	packetsSent atomic.Uint64
	batchesRead atomic.Uint64
	batchesSent atomic.Uint64

	// Timing
	startTime      time.Time
	lastUpdateTime time.Time

	// Peak rates (bytes/sec)
	peakReadRate  atomic.Uint64
	peakWriteRate atomic.Uint64
}

func init() {
	GlobalStats.startTime = time.Now()
	GlobalStats.lastUpdateTime = time.Now()
}

// RecordRead records network read statistics
func (ns *NetworkStats) RecordRead(networkBytes uint64, originalBytes uint64, packets uint64) {
	ns.bytesRead.Add(networkBytes)
	ns.origBytesRead.Add(originalBytes)
	ns.packetsRead.Add(packets)
	ns.batchesRead.Add(1)
	ns.updatePeakRate(networkBytes, true)
}

// RecordWrite records network write statistics
func (ns *NetworkStats) RecordWrite(networkBytes uint64, originalBytes uint64, packets uint64) {
	ns.bytesWritten.Add(networkBytes)
	ns.origBytesSent.Add(originalBytes)
	ns.packetsSent.Add(packets)
	ns.batchesSent.Add(1)
	ns.updatePeakRate(networkBytes, false)
}

func (ns *NetworkStats) updatePeakRate(bytes uint64, isRead bool) {
	now := time.Now()
	elapsed := now.Sub(ns.lastUpdateTime).Seconds()
	if elapsed > 0 {
		rate := float64(bytes) / elapsed
		if isRead {
			currentPeak := ns.peakReadRate.Load()
			if uint64(rate) > currentPeak {
				ns.peakReadRate.Store(uint64(rate))
			}
		} else {
			currentPeak := ns.peakWriteRate.Load()
			if uint64(rate) > currentPeak {
				ns.peakWriteRate.Store(uint64(rate))
			}
		}
	}
	ns.lastUpdateTime = now
}

// GetReport returns a comprehensive network statistics report
func (ns *NetworkStats) GetReport() NetworkReport {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	now := time.Now()
	elapsed := now.Sub(ns.startTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1 // Avoid division by zero
	}

	bytesRead := ns.bytesRead.Load()
	bytesWritten := ns.bytesWritten.Load()
	origBytesRead := ns.origBytesRead.Load()
	origBytesSent := ns.origBytesSent.Load()

	// Calculate compression savings (percentage of bandwidth saved)
	var readCompressionSavings, writeCompressionSavings float64
	if origBytesRead > 0 {
		readCompressionSavings = float64(origBytesRead-bytesRead) / float64(origBytesRead) * 100
	}
	if origBytesSent > 0 {
		writeCompressionSavings = float64(origBytesSent-bytesWritten) / float64(origBytesSent) * 100
	}

	// Calculate compression ratios (original/compressed)
	var readCompressionRatio, writeCompressionRatio float64
	if bytesRead > 0 {
		readCompressionRatio = float64(origBytesRead) / float64(bytesRead)
	}
	if bytesWritten > 0 {
		writeCompressionRatio = float64(origBytesSent) / float64(bytesWritten)
	}

	return NetworkReport{
		// Byte statistics
		TotalReadMB:       float64(bytesRead) * ByteToMBMultiplier,
		TotalWrittenMB:    float64(bytesWritten) * ByteToMBMultiplier,
		OriginalReadMB:    float64(origBytesRead) * ByteToMBMultiplier,
		OriginalWrittenMB: float64(origBytesSent) * ByteToMBMultiplier,

		// Compression metrics
		ReadCompressionRatio:    readCompressionRatio,
		WriteCompressionRatio:   writeCompressionRatio,
		ReadCompressionSavings:  readCompressionSavings,
		WriteCompressionSavings: writeCompressionSavings,

		// Rate statistics
		AvgReadRateKBPerSec:   float64(bytesRead) * ByteToKBMultiplier / elapsed,
		AvgWriteRateKBPerSec:  float64(bytesWritten) * ByteToKBMultiplier / elapsed,
		PeakReadRateKBPerSec:  float64(ns.peakReadRate.Load()) * ByteToKBMultiplier,
		PeakWriteRateKBPerSec: float64(ns.peakWriteRate.Load()) * ByteToKBMultiplier,

		// Packet statistics
		PacketsRead: ns.packetsRead.Load(),
		PacketsSent: ns.packetsSent.Load(),
		BatchesRead: ns.batchesRead.Load(),
		BatchesSent: ns.batchesSent.Load(),

		// Timing
		ElapsedTime: elapsed,
		StartTime:   ns.startTime,
	}
}

// Reset resets all statistics
func (ns *NetworkStats) Reset() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.bytesRead.Store(0)
	ns.bytesWritten.Store(0)
	ns.origBytesRead.Store(0)
	ns.origBytesSent.Store(0)
	ns.packetsRead.Store(0)
	ns.packetsSent.Store(0)
	ns.batchesRead.Store(0)
	ns.batchesSent.Store(0)
	ns.peakReadRate.Store(0)
	ns.peakWriteRate.Store(0)
	ns.startTime = time.Now()
	ns.lastUpdateTime = time.Now()
}

// Legacy functions for backward compatibility
func GetNetworkReport() networkReport {
	report := GlobalStats.GetReport()
	return networkReport{
		CompressionRatioIn:   report.ReadCompressionRatio,
		CompressionRatioOut:  report.WriteCompressionRatio,
		WrittenMB:            report.TotalWrittenMB,
		ReadMB:               report.TotalReadMB,
		AvgWriteRateKBPerSec: report.AvgWriteRateKBPerSec,
		AvgReadRateKBPerSec:  report.AvgReadRateKBPerSec,
		ElapsedTime:          time.Duration(report.ElapsedTime * float64(time.Second)),
	}
}

type NetworkOpts struct {
	FlushRate    time.Duration
	CmpThreshold uint32
}

// NetworkReport contains comprehensive network statistics
type NetworkReport struct {
	// Byte statistics
	TotalReadMB       float64
	TotalWrittenMB    float64
	OriginalReadMB    float64
	OriginalWrittenMB float64

	// Compression metrics
	ReadCompressionRatio    float64 // Original bytes / compressed bytes
	WriteCompressionRatio   float64
	ReadCompressionSavings  float64 // Percentage saved
	WriteCompressionSavings float64

	// Rate statistics
	AvgReadRateKBPerSec   float64
	AvgWriteRateKBPerSec  float64
	PeakReadRateKBPerSec  float64
	PeakWriteRateKBPerSec float64

	// Packet statistics
	PacketsRead uint64
	PacketsSent uint64
	BatchesRead uint64
	BatchesSent uint64

	// Timing
	ElapsedTime float64 // in seconds
	StartTime   time.Time
}

// Legacy type for backward compatibility
type networkReport struct {
	CompressionRatioIn   float64
	CompressionRatioOut  float64
	WrittenMB            float64
	ReadMB               float64
	AvgWriteRateKBPerSec float64
	AvgReadRateKBPerSec  float64
	ElapsedTime          time.Duration
}

// PrintStats prints a formatted summary of network statistics
func (ns *NetworkStats) PrintStats() {
	report := ns.GetReport()
	fmt.Printf("\n=== Network Statistics ===\n")
	fmt.Printf("Runtime: %.1fs\n", report.ElapsedTime)
	fmt.Printf("\n--- Data Transfer ---\n")
	fmt.Printf("Read:    %.2f MB (%.2f MB original)\n", report.TotalReadMB, report.OriginalReadMB)
	fmt.Printf("Written: %.2f MB (%.2f MB original)\n", report.TotalWrittenMB, report.OriginalWrittenMB)

	if report.ReadCompressionSavings > 0 || report.WriteCompressionSavings > 0 {
		fmt.Printf("\n--- Compression ---\n")
		if report.ReadCompressionSavings > 0 {
			fmt.Printf("Read Savings:  %.1f%% (ratio: %.2fx)\n",
				report.ReadCompressionSavings, report.ReadCompressionRatio)
		}
		if report.WriteCompressionSavings > 0 {
			fmt.Printf("Write Savings: %.1f%% (ratio: %.2fx)\n",
				report.WriteCompressionSavings, report.WriteCompressionRatio)
		}
	}

	fmt.Printf("\n--- Rates ---\n")
	fmt.Printf("Avg Read:  %.1f KB/s\n", report.AvgReadRateKBPerSec)
	fmt.Printf("Avg Write: %.1f KB/s\n", report.AvgWriteRateKBPerSec)
	if report.PeakReadRateKBPerSec > 0 {
		fmt.Printf("Peak Read:  %.1f KB/s\n", report.PeakReadRateKBPerSec)
	}
	if report.PeakWriteRateKBPerSec > 0 {
		fmt.Printf("Peak Write: %.1f KB/s\n", report.PeakWriteRateKBPerSec)
	}

	fmt.Printf("\n--- Packets ---\n")
	fmt.Printf("Read:    %d packets (%d batches)\n", report.PacketsRead, report.BatchesRead)
	fmt.Printf("Written: %d packets (%d batches)\n", report.PacketsSent, report.BatchesSent)
	fmt.Printf("========================\n\n")
}
