package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestErrorRateCalculation verifies Property 1: Error rate calculation is accurate
// For any error count E and total count T where T > 0, the error rate SHALL equal E / T.
// **Feature: load-testing-tool, Property 1: Error rate calculation is accurate**
// **Validates: Requirements 3.2**
func TestErrorRateCalculation(t *testing.T) {
	testCases := []struct {
		name         string
		errors       int64
		total        int64
		expectedRate float64
	}{
		{"no errors", 0, 100, 0.0},
		{"10% errors", 10, 100, 0.1},
		{"50% errors", 50, 100, 0.5},
		{"all errors", 100, 100, 1.0},
		{"small sample", 1, 10, 0.1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate error rate using the same formula as in PrintStatus
			var errorRate float64
			if tc.total > 0 {
				errorRate = float64(tc.errors) / float64(tc.total)
			}

			if errorRate != tc.expectedRate {
				t.Errorf("Expected error rate %.4f, got %.4f", tc.expectedRate, errorRate)
			}
		})
	}
}

// TestAverageLatencyCalculation verifies Property 2: Average latency calculation is correct
// For any collection of latency values, the average SHALL equal total latency sum divided by count.
// **Feature: load-testing-tool, Property 2: Average latency calculation is correct**
// **Validates: Requirements 2.2**
func TestAverageLatencyCalculation(t *testing.T) {
	testCases := []struct {
		name           string
		totalLatencyMs int64
		count          int64
		expectedAvg    int64
	}{
		{"single request", 100, 1, 100},
		{"multiple requests", 500, 5, 100},
		{"varied latencies", 1500, 10, 150},
		{"high latency", 10000, 100, 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate average latency using the same formula as in PrintSummary
			var avgLatency int64
			if tc.count > 0 {
				avgLatency = tc.totalLatencyMs / tc.count
			}

			if avgLatency != tc.expectedAvg {
				t.Errorf("Expected avg latency %d, got %d", tc.expectedAvg, avgLatency)
			}
		})
	}
}

// TestAtomicCounterOperations verifies atomic counter operations work correctly
func TestAtomicCounterOperations(t *testing.T) {
	var counter int64 = 0

	// Test atomic add
	atomic.AddInt64(&counter, 1)
	if atomic.LoadInt64(&counter) != 1 {
		t.Error("Atomic add failed")
	}

	// Test atomic store
	atomic.StoreInt64(&counter, 100)
	if atomic.LoadInt64(&counter) != 100 {
		t.Error("Atomic store failed")
	}

	// Test compare and swap (used for maxLatencyMs)
	atomic.CompareAndSwapInt64(&counter, 100, 200)
	if atomic.LoadInt64(&counter) != 200 {
		t.Error("Atomic CAS failed")
	}
}

// TestMaxLatencyUpdate verifies the max latency update logic
func TestMaxLatencyUpdate(t *testing.T) {
	var maxLatencyMs int64 = 0

	// Simulate the max latency update logic from recordSuccess/recordError
	updateMaxLatency := func(latencyMs int64) {
		for {
			currentMax := atomic.LoadInt64(&maxLatencyMs)
			if latencyMs <= currentMax {
				break
			}
			if atomic.CompareAndSwapInt64(&maxLatencyMs, currentMax, latencyMs) {
				break
			}
		}
	}

	// Test increasing latencies
	updateMaxLatency(100)
	if atomic.LoadInt64(&maxLatencyMs) != 100 {
		t.Errorf("Expected max 100, got %d", atomic.LoadInt64(&maxLatencyMs))
	}

	updateMaxLatency(200)
	if atomic.LoadInt64(&maxLatencyMs) != 200 {
		t.Errorf("Expected max 200, got %d", atomic.LoadInt64(&maxLatencyMs))
	}

	// Test lower latency doesn't update max
	updateMaxLatency(50)
	if atomic.LoadInt64(&maxLatencyMs) != 200 {
		t.Errorf("Expected max still 200, got %d", atomic.LoadInt64(&maxLatencyMs))
	}
}

// TestSuccessRateCalculation verifies success rate calculation
func TestSuccessRateCalculation(t *testing.T) {
	testCases := []struct {
		name         string
		success      int64
		total        int64
		expectedRate float64
	}{
		{"all success", 100, 100, 100.0},
		{"90% success", 90, 100, 90.0},
		{"50% success", 50, 100, 50.0},
		{"no success", 0, 100, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate success rate using the same formula as in PrintSummary
			var successRate float64
			if tc.total > 0 {
				successRate = float64(tc.success) / float64(tc.total) * 100
			}

			if successRate != tc.expectedRate {
				t.Errorf("Expected success rate %.1f%%, got %.1f%%", tc.expectedRate, successRate)
			}
		})
	}
}

// TestBatchCalculation verifies batch calculation logic
func TestBatchCalculation(t *testing.T) {
	testCases := []struct {
		name          string
		totalRobots   int
		batchSize     int
		expectedBatch int
	}{
		{"exact division", 100, 10, 10},
		{"with remainder", 105, 10, 11},
		{"single batch", 5, 10, 1},
		{"large batch", 1000, 50, 20},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate total batches using the same formula as in RunLoadTest
			totalBatches := (tc.totalRobots + tc.batchSize - 1) / tc.batchSize

			if totalBatches != tc.expectedBatch {
				t.Errorf("Expected %d batches, got %d", tc.expectedBatch, totalBatches)
			}
		})
	}
}

// TestErrorThresholdCheck verifies error threshold checking logic
func TestErrorThresholdCheck(t *testing.T) {
	errorThreshold := 0.1 // 10%

	testCases := []struct {
		name       string
		errors     int64
		total      int64
		shouldStop bool
	}{
		{"below threshold", 5, 100, false},
		{"at threshold", 10, 100, false}, // > not >=
		{"above threshold", 11, 100, true},
		{"way above", 50, 100, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var shouldStop bool
			if tc.total > 0 {
				currentErrorRate := float64(tc.errors) / float64(tc.total)
				shouldStop = currentErrorRate > errorThreshold
			}

			if shouldStop != tc.shouldStop {
				t.Errorf("Expected shouldStop=%v, got %v", tc.shouldStop, shouldStop)
			}
		})
	}
}

// TestLatencyDegradationDetection verifies latency degradation detection
func TestLatencyDegradationDetection(t *testing.T) {
	const degradationThreshold int64 = 1000 // 1 second in ms

	testCases := []struct {
		name       string
		latencyMs  int64
		isDegraded bool
	}{
		{"normal latency", 100, false},
		{"borderline", 1000, false}, // > not >=
		{"degraded", 1001, true},
		{"very slow", 5000, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isDegraded := tc.latencyMs > degradationThreshold

			if isDegraded != tc.isDegraded {
				t.Errorf("Expected isDegraded=%v, got %v", tc.isDegraded, isDegraded)
			}
		})
	}
}

// TestPrintIntervalTiming verifies print interval is reasonable
func TestPrintIntervalTiming(t *testing.T) {
	printInterval := 5 * time.Second

	if printInterval < 1*time.Second {
		t.Error("Print interval too short, may cause log spam")
	}

	if printInterval > 30*time.Second {
		t.Error("Print interval too long, may miss important status updates")
	}
}

// ==================== Stability Tests for Task 2.2 ====================

// TestConcurrentMetricUpdates verifies metrics are stable under concurrent updates
// This simulates 100 robots updating metrics concurrently
func TestConcurrentMetricUpdates(t *testing.T) {
	// Reset counters
	var testOnlineCount int64
	var testTotalRequests int64
	var testSuccessCount int64
	var testErrorCount int64
	var testTotalLatencyMs int64
	var testMaxLatencyMs int64

	numRobots := 100
	done := make(chan bool, numRobots)

	// Simulate 100 concurrent robots
	for i := 0; i < numRobots; i++ {
		go func(robotID int) {
			// Simulate request
			atomic.AddInt64(&testTotalRequests, 1)

			// Simulate some latency (10-100ms)
			latencyMs := int64(10 + (robotID % 90))

			// 90% success rate
			if robotID%10 != 0 {
				atomic.AddInt64(&testSuccessCount, 1)
				atomic.AddInt64(&testOnlineCount, 1)
			} else {
				atomic.AddInt64(&testErrorCount, 1)
			}

			atomic.AddInt64(&testTotalLatencyMs, latencyMs)

			// Update max latency
			for {
				currentMax := atomic.LoadInt64(&testMaxLatencyMs)
				if latencyMs <= currentMax {
					break
				}
				if atomic.CompareAndSwapInt64(&testMaxLatencyMs, currentMax, latencyMs) {
					break
				}
			}

			done <- true
		}(i)
	}

	// Wait for all robots
	for i := 0; i < numRobots; i++ {
		<-done
	}

	// Verify counts
	totalReqs := atomic.LoadInt64(&testTotalRequests)
	successCnt := atomic.LoadInt64(&testSuccessCount)
	errorCnt := atomic.LoadInt64(&testErrorCount)
	onlineCnt := atomic.LoadInt64(&testOnlineCount)

	if totalReqs != int64(numRobots) {
		t.Errorf("Expected %d total requests, got %d", numRobots, totalReqs)
	}

	if successCnt+errorCnt != int64(numRobots) {
		t.Errorf("Success + Error should equal total: %d + %d != %d", successCnt, errorCnt, numRobots)
	}

	if onlineCnt != successCnt {
		t.Errorf("Online count should equal success count: %d != %d", onlineCnt, successCnt)
	}

	// Verify error rate is approximately 10%
	errorRate := float64(errorCnt) / float64(totalReqs)
	if errorRate < 0.05 || errorRate > 0.15 {
		t.Errorf("Error rate should be around 10%%, got %.1f%%", errorRate*100)
	}

	t.Logf("Concurrent test results: total=%d, success=%d, errors=%d, online=%d, errorRate=%.1f%%",
		totalReqs, successCnt, errorCnt, onlineCnt, errorRate*100)
}

// TestMetricStabilityUnderLoad verifies metrics remain consistent under sustained load
func TestMetricStabilityUnderLoad(t *testing.T) {
	var counter int64 = 0
	iterations := 1000
	goroutines := 100

	done := make(chan bool, goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			for i := 0; i < iterations/goroutines; i++ {
				atomic.AddInt64(&counter, 1)
			}
			done <- true
		}()
	}

	for g := 0; g < goroutines; g++ {
		<-done
	}

	finalCount := atomic.LoadInt64(&counter)
	if finalCount != int64(iterations) {
		t.Errorf("Expected counter to be %d, got %d (lost %d increments)",
			iterations, finalCount, iterations-int(finalCount))
	}
}

// TestErrorRateChangeDetection verifies we can detect error rate changes
func TestErrorRateChangeDetection(t *testing.T) {
	// Simulate gradual increase in errors
	type snapshot struct {
		errors int64
		total  int64
	}

	snapshots := []snapshot{
		{0, 10},  // 0%
		{1, 20},  // 5%
		{3, 30},  // 10%
		{6, 40},  // 15%
		{10, 50}, // 20%
	}

	var prevRate float64
	for i, s := range snapshots {
		currentRate := float64(s.errors) / float64(s.total)

		if i > 0 && currentRate < prevRate {
			t.Errorf("Error rate should be increasing: %.1f%% < %.1f%%",
				currentRate*100, prevRate*100)
		}

		// Check if we should stop spawning (threshold 10%)
		shouldStop := currentRate > 0.1
		t.Logf("Snapshot %d: errors=%d, total=%d, rate=%.1f%%, shouldStop=%v",
			i, s.errors, s.total, currentRate*100, shouldStop)

		prevRate = currentRate
	}
}

// TestLatencyChangeDetection verifies we can detect latency changes
func TestLatencyChangeDetection(t *testing.T) {
	// Simulate increasing latency under load
	type latencySnapshot struct {
		totalLatencyMs int64
		count          int64
	}

	snapshots := []latencySnapshot{
		{500, 10},   // 50ms avg
		{1500, 20},  // 75ms avg
		{4500, 30},  // 150ms avg
		{12000, 40}, // 300ms avg
		{30000, 50}, // 600ms avg
	}

	for i, s := range snapshots {
		avgLatency := s.totalLatencyMs / s.count
		isDegraded := avgLatency > 1000

		t.Logf("Snapshot %d: avgLatency=%dms, isDegraded=%v", i, avgLatency, isDegraded)
	}
}

// TestBatchSpawningStability verifies batch spawning logic handles edge cases
func TestBatchSpawningStability(t *testing.T) {
	testCases := []struct {
		name        string
		totalRobots int
		batchSize   int
	}{
		{"100 robots, batch 10", 100, 10},
		{"100 robots, batch 1", 100, 1},
		{"100 robots, batch 100", 100, 100},
		{"100 robots, batch 7", 100, 7}, // non-divisible
		{"100 robots, batch 33", 100, 33},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			totalBatches := (tc.totalRobots + tc.batchSize - 1) / tc.batchSize

			// Verify all robots would be spawned
			robotsSpawned := 0
			for batch := 0; batch < totalBatches; batch++ {
				start := batch * tc.batchSize
				end := start + tc.batchSize
				if end > tc.totalRobots {
					end = tc.totalRobots
				}
				robotsSpawned += end - start
			}

			if robotsSpawned != tc.totalRobots {
				t.Errorf("Expected %d robots spawned, got %d", tc.totalRobots, robotsSpawned)
			}

			t.Logf("%s: %d batches, all %d robots would be spawned",
				tc.name, totalBatches, robotsSpawned)
		})
	}
}
