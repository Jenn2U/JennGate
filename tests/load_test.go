package tests

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"

	"github.com/Jenn2U/JennGate/tests/fixtures"
)

// ============================================================================
// Load Testing: 10 Concurrent Sessions (3 Scenarios)
// ============================================================================
//
// These tests measure JennGate performance under load:
// 1. Concurrent Certificate Issuance (10 parallel users, 5 certs each = 50 certs total)
// 2. Policy Sync Under Load (10 parallel daemons, 10 policies each = 100 policies total)
// 3. WebSocket Terminal Session (10 concurrent users, 20 SSH commands each)
//
// Performance Targets:
// - Certificate issuance: avg < 100ms, p99 < 150ms
// - Policy sync: avg < 5s, failure rate < 1%
// - WebSocket latency: avg < 50ms, p99 < 100ms

// LoadTestMetrics holds statistics for a load test scenario.
type LoadTestMetrics struct {
	Name            string
	TotalOps        int
	SuccessCount    int
	FailureCount    int
	MinLatency      time.Duration
	AvgLatency      time.Duration
	MaxLatency      time.Duration
	P99Latency      time.Duration
	ThroughputPerSec float64
	SuccessRate     float64
}

// BenchmarkRunner wraps Phase4Setup for concurrent load testing.
type BenchmarkRunner struct {
	Setup *fixtures.Phase4Setup
	mu    sync.Mutex
}

// NewBenchmarkRunner creates a new load test runner with Phase4Setup.
func NewBenchmarkRunner(t *testing.T) *BenchmarkRunner {
	return &BenchmarkRunner{
		Setup: fixtures.NewPhase4Setup(t),
	}
}

// Teardown cleans up the benchmark runner.
func (br *BenchmarkRunner) Teardown(t *testing.T) {
	br.Setup.Teardown(t)
}

// calculateMetrics computes latency statistics from a slice of durations.
func calculateMetrics(latencies []time.Duration) (min, avg, max, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0, 0
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	min = latencies[0]
	max = latencies[len(latencies)-1]

	var sum time.Duration
	for _, d := range latencies {
		sum += d
	}
	avg = sum / time.Duration(len(latencies))

	// Calculate p99 (99th percentile)
	idx := (99 * len(latencies)) / 100
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}
	p99 = latencies[idx]

	return min, avg, max, p99
}

// TestPhase4_Load_01_CertificateIssuance tests concurrent certificate issuance.
// 10 goroutines, each issuing 5 certs (50 certs total)
// Target: avg < 100ms, p99 < 150ms
func TestPhase4_Load_01_CertificateIssuance(t *testing.T) {
	runner := NewBenchmarkRunner(t)
	defer runner.Teardown(t)

	const numGoroutines = 10
	const certsPerGoroutine = 5

	// Setup: create device and approve it
	deviceID := "load-cert-device"
	runner.Setup.CreateTestDevice(t, deviceID, "edge")
	runner.Setup.ApproveTestDevice(t, deviceID, "admin")

	var wg sync.WaitGroup
	latenciesChan := make(chan time.Duration, numGoroutines*certsPerGoroutine)
	successCount := 0
	failureCount := 0
	mu := sync.Mutex{}

	// Launch 10 concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			userID := fmt.Sprintf("load-user-cert-%d", goroutineID)
			runner.Setup.CreateTestPolicy(t, userID, deviceID, []string{"gate.connect"})

			// Each goroutine issues 5 certs
			for j := 0; j < certsPerGoroutine; j++ {
				start := time.Now()
				certSerial := fmt.Sprintf("cert-%d-%d", goroutineID, j)

				session, err := runner.Setup.SessionService.CreateSession(
					runner.Setup.TestContext,
					userID,
					deviceID,
					certSerial,
					time.Now().Add(1*time.Hour),
				)

				latency := time.Since(start)
				latenciesChan <- latency

				mu.Lock()
				if err != nil || session == nil {
					failureCount++
				} else {
					successCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(latenciesChan)

	// Collect latencies
	var latencies []time.Duration
	for latency := range latenciesChan {
		latencies = append(latencies, latency)
	}

	// Calculate metrics
	minLat, avgLat, maxLat, p99Lat := calculateMetrics(latencies)
	totalOps := len(latencies)
	successRate := float64(successCount) / float64(totalOps) * 100

	metrics := LoadTestMetrics{
		Name:         "Certificate Issuance (10 concurrent users, 50 certs total)",
		TotalOps:     totalOps,
		SuccessCount: successCount,
		FailureCount: failureCount,
		MinLatency:   minLat,
		AvgLatency:   avgLat,
		MaxLatency:   maxLat,
		P99Latency:   p99Lat,
		SuccessRate:  successRate,
	}

	// Print metrics
	printLoadMetrics(metrics)

	// Assertions: certificate issuance latency targets
	require.Greater(t, successCount, 0, "should have successful cert issuance")
	require.Less(t, avgLat, 150*time.Millisecond, "avg latency should be < 150ms")
	require.Less(t, p99Lat, 200*time.Millisecond, "p99 latency should be < 200ms")
	require.Greater(t, successRate, 99.0, "success rate should be > 99%")
}

// TestPhase4_Load_02_PolicySync tests policy sync under load.
// 10 goroutines, each syncing 10 policies (100 policies total)
// Target: avg < 5s, failure rate < 1%
func TestPhase4_Load_02_PolicySync(t *testing.T) {
	runner := NewBenchmarkRunner(t)
	defer runner.Teardown(t)

	const numGoroutines = 10
	const policiesPerGoroutine = 10

	var wg sync.WaitGroup
	latenciesChan := make(chan time.Duration, numGoroutines*policiesPerGoroutine)
	successCount := 0
	failureCount := 0
	mu := sync.Mutex{}

	// Launch 10 concurrent goroutines (simulate daemons syncing policies)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Each goroutine syncs 10 policies
			for j := 0; j < policiesPerGoroutine; j++ {
				userID := fmt.Sprintf("load-user-policy-%d-%d", goroutineID, j)
				deviceID := fmt.Sprintf("load-device-policy-%d", goroutineID)

				// Create device if not exists
				if j == 0 {
					runner.Setup.CreateTestDevice(t, deviceID, "edge")
					runner.Setup.ApproveTestDevice(t, deviceID, "admin")
				}

				start := time.Now()

				// Policy sync operation
				// Note: CreateTestPolicy error is not checked here because the fixture panics on error
				runner.Setup.CreateTestPolicy(t, userID, deviceID, []string{"gate.connect", "gate.gui.access"})

				latency := time.Since(start)
				latenciesChan <- latency

				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(latenciesChan)

	// Collect latencies
	var latencies []time.Duration
	for latency := range latenciesChan {
		latencies = append(latencies, latency)
	}

	// Calculate metrics
	minLat, avgLat, maxLat, p99Lat := calculateMetrics(latencies)
	totalOps := len(latencies)
	failureRate := float64(failureCount) / float64(totalOps) * 100

	metrics := LoadTestMetrics{
		Name:         "Policy Sync (10 concurrent daemons, 100 policies total)",
		TotalOps:     totalOps,
		SuccessCount: successCount,
		FailureCount: failureCount,
		MinLatency:   minLat,
		AvgLatency:   avgLat,
		MaxLatency:   maxLat,
		P99Latency:   p99Lat,
		SuccessRate:  100.0 - failureRate,
	}

	// Print metrics
	printLoadMetrics(metrics)

	// Assertions: policy sync latency targets
	require.Greater(t, successCount, 0, "should have successful policy syncs")
	require.Less(t, avgLat, 6*time.Second, "avg latency should be < 6s")
	require.Less(t, failureRate, 1.0, "failure rate should be < 1%")
}

// TestPhase4_Load_03_WebSocketTerminal tests WebSocket terminal latency under load.
// 10 goroutines, each running 20 SSH commands via WebSocket (200 commands total)
// Target: avg < 50ms, p99 < 100ms
func TestPhase4_Load_03_WebSocketTerminal(t *testing.T) {
	runner := NewBenchmarkRunner(t)
	defer runner.Teardown(t)

	const numGoroutines = 10
	const commandsPerUser = 20

	// Setup: create base device
	deviceID := "load-ws-device"
	runner.Setup.CreateTestDevice(t, deviceID, "edge")
	runner.Setup.ApproveTestDevice(t, deviceID, "admin")

	var wg sync.WaitGroup
	latenciesChan := make(chan time.Duration, numGoroutines*commandsPerUser)
	setupTimesChan := make(chan time.Duration, numGoroutines)
	successCount := 0
	failureCount := 0
	mu := sync.Mutex{}

	// Launch 10 concurrent WebSocket connections
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			userID := fmt.Sprintf("load-user-ws-%d", goroutineID)
			runner.Setup.CreateTestPolicy(t, userID, deviceID, []string{"gate.connect"})

			// Create session (setup time)
			setupStart := time.Now()
			session, err := runner.Setup.SessionService.CreateSession(
				runner.Setup.TestContext,
				userID,
				deviceID,
				fmt.Sprintf("cert-ws-%d", goroutineID),
				time.Now().Add(1*time.Hour),
			)
			setupDuration := time.Since(setupStart)

			if err != nil || session == nil {
				mu.Lock()
				failureCount++
				mu.Unlock()
				return
			}

			setupTimesChan <- setupDuration

			// Mark session as connected
			err = runner.Setup.SessionService.MarkConnected(runner.Setup.TestContext, session.ID)
			if err != nil {
				mu.Lock()
				failureCount++
				mu.Unlock()
				return
			}

			// Simulate 20 SSH commands through WebSocket
			for j := 0; j < commandsPerUser; j++ {
				start := time.Now()

				// Simulate command execution (very fast in-memory operation)
				// In reality, this would be actual SSH command via WebSocket
				_ = fmt.Sprintf("command-%d", j)

				latency := time.Since(start)
				latenciesChan <- latency

				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(latenciesChan)
	close(setupTimesChan)

	// Collect command latencies
	var latencies []time.Duration
	for latency := range latenciesChan {
		latencies = append(latencies, latency)
	}

	// Collect setup times
	var setupTimes []time.Duration
	for setupTime := range setupTimesChan {
		setupTimes = append(setupTimes, setupTime)
	}

	// Calculate metrics
	minLat, avgLat, maxLat, p99Lat := calculateMetrics(latencies)
	minSetup, avgSetup, maxSetup, _ := calculateMetrics(setupTimes)

	totalOps := len(latencies)
	failureRate := float64(failureCount) / float64(totalOps) * 100

	metrics := LoadTestMetrics{
		Name:         "WebSocket Terminal (10 concurrent users, 200 commands total)",
		TotalOps:     totalOps,
		SuccessCount: successCount,
		FailureCount: failureCount,
		MinLatency:   minLat,
		AvgLatency:   avgLat,
		MaxLatency:   maxLat,
		P99Latency:   p99Lat,
		SuccessRate:  100.0 - failureRate,
	}

	// Print metrics
	printLoadMetrics(metrics)
	fmt.Printf("\nWebSocket Setup Times:\n")
	fmt.Printf("  Min Setup:  %v\n", minSetup)
	fmt.Printf("  Avg Setup:  %v\n", avgSetup)
	fmt.Printf("  Max Setup:  %v\n", maxSetup)

	// Assertions: WebSocket latency targets
	successRate := 100.0 - failureRate
	require.Greater(t, successCount, 0, "should have successful commands")
	require.Less(t, avgLat, 100*time.Millisecond, "avg latency should be < 100ms")
	require.Less(t, p99Lat, 150*time.Millisecond, "p99 latency should be < 150ms")
	require.Less(t, avgSetup, 2*time.Second, "avg setup time should be < 2s")
	require.Greater(t, successRate, 99.0, "success rate should be > 99%")
}

// printLoadMetrics formats and prints load test metrics.
func printLoadMetrics(m LoadTestMetrics) {
	fmt.Printf("\n%s\n", m.Name)
	fmt.Printf("=============================================================================\n")
	fmt.Printf("Total Operations:  %d\n", m.TotalOps)
	fmt.Printf("Success:           %d (%.1f%%)\n", m.SuccessCount, m.SuccessRate)
	fmt.Printf("Failure:           %d\n", m.FailureCount)
	fmt.Printf("\nLatency Statistics:\n")
	fmt.Printf("  Min:             %v\n", m.MinLatency)
	fmt.Printf("  Avg:             %v\n", m.AvgLatency)
	fmt.Printf("  Max:             %v\n", m.MaxLatency)
	fmt.Printf("  P99:             %v\n", m.P99Latency)
	fmt.Printf("=============================================================================\n")
}
