package host

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"nex-lang/pkg/evaluator"
)

// processStarted is used for uptime gauge.
var processStarted = time.Now()

type hostMetrics struct {
	requestsTotal atomic.Int64
	requests2xx   atomic.Int64
	requests4xx   atomic.Int64
	requests5xx   atomic.Int64
	publishTotal  atomic.Int64
	downloadTotal atomic.Int64
}

func (h *Host) observeRequest(status int) {
	if h == nil {
		return
	}
	h.metrics.requestsTotal.Add(1)
	switch {
	case status >= 500:
		h.metrics.requests5xx.Add(1)
	case status >= 400:
		h.metrics.requests4xx.Add(1)
	case status >= 200 && status < 300:
		h.metrics.requests2xx.Add(1)
	}
}

// IncPublish increments the successful publish counter.
func (h *Host) IncPublish() {
	if h == nil {
		return
	}
	h.metrics.publishTotal.Add(1)
}

// IncDownload increments the download response counter.
func (h *Host) IncDownload() {
	if h == nil {
		return
	}
	h.metrics.downloadTotal.Add(1)
}

// PrometheusText renders Prometheus exposition format metrics.
func (h *Host) PrometheusText() string {
	var b strings.Builder
	uptime := time.Since(processStarted).Seconds()

	write := func(name, help, typ string, value any) {
		fmt.Fprintf(&b, "# HELP %s %s\n", name, help)
		fmt.Fprintf(&b, "# TYPE %s %s\n", name, typ)
		fmt.Fprintf(&b, "%s %v\n", name, value)
	}

	write("nex_registry_up", "1 if the process is serving", "gauge", 1)
	write("nex_registry_uptime_seconds", "Seconds since process start", "gauge", fmt.Sprintf("%.3f", uptime))
	write("nex_registry_http_requests_total", "Total HTTP requests handled", "counter", h.metrics.requestsTotal.Load())
	write("nex_registry_http_responses_2xx_total", "HTTP responses with 2xx status", "counter", h.metrics.requests2xx.Load())
	write("nex_registry_http_responses_4xx_total", "HTTP responses with 4xx status", "counter", h.metrics.requests4xx.Load())
	write("nex_registry_http_responses_5xx_total", "HTTP responses with 5xx status", "counter", h.metrics.requests5xx.Load())
	write("nex_registry_publishes_total", "Successful package publishes", "counter", h.metrics.publishTotal.Load())
	write("nex_registry_downloads_total", "Package download responses served", "counter", h.metrics.downloadTotal.Load())
	write("nex_registry_routes_registered", "Nexus routes registered at load time", "gauge", h.RouteCount)

	if h.DB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if n, err := h.DB.CountPackages(ctx); err == nil {
			write("nex_registry_packages", "Packages currently in the registry", "gauge", n)
		}
		if n, err := h.DB.CountVersions(ctx); err == nil {
			write("nex_registry_versions", "Published versions currently in the registry", "gauge", n)
		}
		if h.DB.HasReadReplica() {
			write("nex_registry_read_replica_configured", "1 if DATABASE_URL_READ is attached", "gauge", 1)
		} else {
			write("nex_registry_read_replica_configured", "1 if DATABASE_URL_READ is attached", "gauge", 0)
		}
	}

	return b.String()
}

func (h *Host) registerMetricsRoutes() {
	h.Router.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(h.PrometheusText()))
	})
}

func (h *Host) registerMetricsBuiltins(b map[string]*evaluator.Builtin) {
	b["prometheus_metrics"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		return &evaluator.String{Value: h.PrometheusText()}
	}}
	b["db_ping"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		// Returns boolean (never Error) so .nex health checks can branch safely.
		if h.DB == nil {
			return evaluator.FALSE
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := h.DB.Ping(ctx); err != nil {
			return evaluator.FALSE
		}
		return evaluator.TRUE
	}}
}
