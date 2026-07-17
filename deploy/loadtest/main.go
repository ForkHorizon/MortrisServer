// Command mortris-loadtest exercises the section 15 capacity acceptance
// criteria: sustained accepted-events/sec at a target rate for a target
// duration, or a short burst, reporting p50/p95/p99 latency and
// accepted/duplicate/rejected counts. Deliberately a separate Go module
// (own go.mod) so it never becomes part of the production dependency
// graph in docs/dependencies.md — this is a dev/ops tool, not shipped code.
//
// Usage:
//
//	go run . -url https://mortris.forkhorizon.com -project my-project \
//	  -rate 250 -duration 30m -installs 50
//
//	go run . -url https://mortris.forkhorizon.com -project my-project \
//	  -rate 1000 -duration 1m -installs 200
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func uuidv4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func credential() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

type installation struct {
	id   string
	cred string
}

func registerInstall(baseURL, projectID string) (*installation, error) {
	inst := &installation{id: uuidv4(), cred: credential()}
	body, _ := json.Marshal(map[string]any{
		"schema_version":          1,
		"project_id":              projectID,
		"install_id":              inst.id,
		"installation_credential": inst.cred,
		"sdk_name":                "loadtest",
		"sdk_version":             "1.0.0",
		"app_version":             "1.0.0",
		"build_number":            "1",
		"platform":                "android",
	})
	resp, err := http.Post(baseURL+"/v1/installs/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("register failed (%d): %s", resp.StatusCode, b)
	}
	return inst, nil
}

func sendBatch(client *http.Client, baseURL, projectID string, inst *installation, batchSize int) (accepted, duplicates, rejected int, latency time.Duration, err error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	sessionID := uuidv4()
	events := make([]map[string]any, batchSize)
	for i := range events {
		events[i] = map[string]any{
			"event_id":                uuidv4(),
			"session_id":              sessionID,
			"sequence":                i + 1,
			"session_elapsed_ms":      i * 1000,
			"name":                    "level_start",
			"occurred_at_client":      now,
			"app_version":             "1.0.0",
			"build_number":            "1",
			"platform":                "android",
			"os_version":              "15",
			"device_class":            "phone",
			"locale":                  "en-US",
			"timezone_offset_minutes": 0,
			"properties":              map[string]any{},
		}
	}
	body, _ := json.Marshal(map[string]any{
		"schema_version": 1,
		"project_id":     projectID,
		"install_id":     inst.id,
		"sdk":            map[string]any{"name": "loadtest", "version": "1.0.0"},
		"sent_at_client": now,
		"events":         events,
	})

	req, _ := http.NewRequest("POST", baseURL+"/v1/events/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+inst.cred)

	start := time.Now()
	resp, err := client.Do(req)
	latency = time.Since(start)
	if err != nil {
		return 0, 0, 0, latency, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, 0, 0, latency, fmt.Errorf("batch failed (%d): %s", resp.StatusCode, b)
	}
	var result struct {
		Accepted   []string `json:"accepted"`
		Duplicates []string `json:"duplicates"`
		Rejected   []any    `json:"rejected"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return 0, 0, 0, latency, err
	}
	return len(result.Accepted), len(result.Duplicates), len(result.Rejected), latency, nil
}

func main() {
	baseURL := flag.String("url", "", "base URL, e.g. https://mortris.forkhorizon.com (required)")
	projectID := flag.String("project", "", "project ID, must already exist (required)")
	rate := flag.Float64("rate", 250, "target accepted events per second")
	duration := flag.Duration("duration", 30*time.Minute, "test duration")
	batchSize := flag.Int("batch-size", 20, "events per batch request")
	numInstalls := flag.Int("installs", 50, "number of installations to register and spread load across")
	flag.Parse()

	if *baseURL == "" || *projectID == "" {
		fmt.Fprintln(os.Stderr, "usage: -url and -project are required")
		os.Exit(2)
	}

	log.Printf("registering %d installations against %s...", *numInstalls, *projectID)
	installs := make([]*installation, 0, *numInstalls)
	for i := 0; i < *numInstalls; i++ {
		inst, err := registerInstall(*baseURL, *projectID)
		if err != nil {
			log.Fatalf("registration %d failed: %v", i, err)
		}
		installs = append(installs, inst)
	}
	log.Printf("registered. starting load: %.0f events/sec target, batch size %d, for %s", *rate, *batchSize, *duration)

	client := &http.Client{Timeout: 10 * time.Second}
	requestsPerSec := *rate / float64(*batchSize)
	interval := time.Duration(float64(time.Second) / requestsPerSec)

	var totalAccepted, totalDuplicates, totalRejected, totalErrors int64
	var latencies []time.Duration
	var latMu sync.Mutex

	stop := time.After(*duration)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var wg sync.WaitGroup
	var installIdx int64

loop:
	for {
		select {
		case <-stop:
			break loop
		case <-ticker.C:
			idx := atomic.AddInt64(&installIdx, 1) % int64(len(installs))
			inst := installs[idx]
			wg.Add(1)
			go func() {
				defer wg.Done()
				a, d, r, lat, err := sendBatch(client, *baseURL, *projectID, inst, *batchSize)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					log.Printf("request error: %v", err)
					return
				}
				atomic.AddInt64(&totalAccepted, int64(a))
				atomic.AddInt64(&totalDuplicates, int64(d))
				atomic.AddInt64(&totalRejected, int64(r))
				latMu.Lock()
				latencies = append(latencies, lat)
				latMu.Unlock()
			}()
		}
	}
	wg.Wait()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	pct := func(p float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		idx := int(float64(len(latencies)) * p)
		if idx >= len(latencies) {
			idx = len(latencies) - 1
		}
		return latencies[idx]
	}

	fmt.Println("\n=== Load test results ===")
	fmt.Printf("accepted=%d duplicates=%d rejected=%d errors=%d\n", totalAccepted, totalDuplicates, totalRejected, totalErrors)
	fmt.Printf("requests=%d\n", len(latencies))
	if len(latencies) > 0 {
		fmt.Printf("latency p50=%s p95=%s p99=%s max=%s\n", pct(0.50), pct(0.95), pct(0.99), latencies[len(latencies)-1])
	}
	actualDuration := (*duration).Seconds()
	fmt.Printf("effective accepted rate: %.1f events/sec (target %.0f)\n", float64(totalAccepted)/actualDuration, *rate)
}
