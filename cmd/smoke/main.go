// Command smoke probes an already running local API with harmless GET requests.
// It never loads .env, authenticates, starts a server, or writes database rows.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type smokeResult struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status,omitempty"`
	RequestID  string `json:"request_id"`
	DurationMS int64  `json:"duration_ms"`
	Result     string `json:"result"`
	Reason     string `json:"reason,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, output, diagnostics io.Writer) int {
	flags := flag.NewFlagSet("smoke", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	baseURL := flags.String("url", "http://127.0.0.1:8081", "local API origin (loopback only)")
	timeout := flags.Duration("timeout", 5*time.Second, "timeout for each probe")
	readiness := flags.Bool("readiness", false, "also check dependency readiness (unavailable is inconclusive)")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	u, err := url.Parse(*baseURL)
	if err != nil || !validOrigin(u) || flags.NArg() != 0 || *timeout <= 0 {
		_, _ = fmt.Fprintln(diagnostics, "smoke requires a local HTTP(S) origin without credentials, path, query, or fragment and a positive timeout")
		return 1
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		_, _ = fmt.Fprintln(diagnostics, "could not create smoke request ID")
		return 1
	}
	prefix := "cuways-smoke-" + hex.EncodeToString(random[:])
	client := &http.Client{
		Timeout: *timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	paths := []string{"/healthz"}
	if *readiness {
		paths = append(paths, "/readyz")
	}
	exitCode := 0
	encoder := json.NewEncoder(output)
	for _, path := range paths {
		result := probe(client, u, path, prefix+"-"+strings.TrimPrefix(path, "/"))
		if err := encoder.Encode(result); err != nil {
			_, _ = fmt.Fprintln(diagnostics, "could not write smoke evidence")
			return 1
		}
		if result.Result == "failed" {
			exitCode = 1
		} else if result.Result == "inconclusive" && exitCode == 0 {
			exitCode = 2
		}
	}
	return exitCode
}

func validOrigin(u *url.URL) bool {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func probe(client *http.Client, origin *url.URL, path, requestID string) (result smokeResult) {
	result = smokeResult{Method: "GET", Path: path, RequestID: requestID, Result: "failed"}
	started := time.Now()
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()
	target := *origin
	target.Path = path
	request, err := http.NewRequest(http.MethodGet, target.String(), nil)
	if err != nil {
		result.Reason = "invalid_request"
		return result
	}
	request.Header.Set("X-Request-ID", requestID)
	res, err := client.Do(request)
	if err != nil {
		result.Result = "inconclusive"
		result.Reason = "server_unreachable"
		return result
	}
	defer func() { _ = res.Body.Close() }()
	result.Status = res.StatusCode
	if res.Header.Get("X-Request-ID") != requestID {
		result.Reason = "request_id_mismatch"
		return result
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4097))
	if err != nil || len(body) > 4096 {
		result.Reason = "invalid_response_body"
		return result
	}
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Status string `json:"status"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		result.Reason = "invalid_response_envelope"
		return result
	}
	if path == "/readyz" && res.StatusCode == http.StatusServiceUnavailable && envelope.Status == "error" && envelope.Error.Code == "database_unavailable" {
		result.Result = "inconclusive"
		result.Reason = "database_unavailable"
		return result
	}
	expected := "ok"
	if path == "/readyz" {
		expected = "ready"
	}
	if res.StatusCode != http.StatusOK || envelope.Status != "success" || envelope.Data.Status != expected {
		result.Reason = "unexpected_probe_contract"
		return result
	}
	result.Result = "passed"
	return result
}
