package main

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type TestConfig struct {
	Frequency int
	Duration  time.Duration
	TargetURL string
	Method    string
}

func main() {
	config := TestConfig{
		Frequency: 1000,
		Duration:  60 * time.Second,
		TargetURL: "http://localhost:8080/counter/1",
		Method:    "GET",
	}

	target := vegeta.Target{
		Method: config.Method,
		URL:    config.TargetURL,
	}

	rate := vegeta.Rate{Freq: config.Frequency, Per: time.Second}
	duration := config.Duration

	targeter := vegeta.NewStaticTargeter(target)
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Test") {
		metrics.Add(res)
	}

	metrics.Close()
	printTestResults(metrics)
}

func printTestResults(metrics vegeta.Metrics) {

	fmt.Printf("99th percentile: %s\n", metrics.Latencies.P99)

	fmt.Printf("Requests: %d\n", metrics.Requests)

	fmt.Printf("Success rate: %.2f%%\n", metrics.Success*100)

	fmt.Printf("Request Latency Stats:\n")
	fmt.Printf("  Average latency: %s\n", metrics.Latencies.Mean)
	fmt.Printf("  Max latency: %s\n", metrics.Latencies.Max)
	fmt.Printf("  Min latency: %s\n", metrics.Latencies.Min)
}
