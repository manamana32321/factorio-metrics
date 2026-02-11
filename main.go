package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Parse config from env
	rconHost := envOrDefault("RCON_HOST", "localhost")
	rconPort := envOrDefault("RCON_PORT", "27015")
	rconPassword := os.Getenv("RCON_PASSWORD")
	interval := parseDuration(envOrDefault("COLLECT_INTERVAL", "15s"))
	luaScript := loadLuaScript(envOrDefault("LUA_SCRIPT_PATH", "/lua/collect.lua"))

	// Init OTel metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to create metric exporter: %v", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(interval))),
	)
	defer meterProvider.Shutdown(ctx)

	// Init OTel log exporter
	logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to create log exporter: %v", err)
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	defer loggerProvider.Shutdown(ctx)

	logger := loggerProvider.Logger("factorio-metrics")

	// Start metrics collector
	collector, err := NewCollector(rconHost, rconPort, rconPassword, luaScript, meterProvider)
	if err != nil {
		log.Fatalf("failed to create collector: %v", err)
	}

	go collector.Run(ctx, interval)

	// Start log tailer
	podLabel := envOrDefault("FACTORIO_POD_LABEL", "app=factorio-factorio-server-charts")
	namespace := envOrDefault("FACTORIO_NAMESPACE", "factorio")
	tailer := NewLogTailer(namespace, podLabel, logger)
	go tailer.Run(ctx)

	log.Printf("factorio-metrics started (interval=%s, rcon=%s:%s)", interval, rconHost, rconPort)
	<-ctx.Done()
	log.Println("shutting down...")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Fatalf("invalid duration %q: %v", s, err)
	}
	return d
}

func loadLuaScript(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read lua script %s: %v", path, err)
	}
	return string(data)
}

// logEvent sends a structured log record via OTel.
func logEvent(logger otellog.Logger, event string, attrs ...otellog.KeyValue) {
	var record otellog.Record
	record.SetBody(otellog.StringValue(event))
	record.AddAttributes(attrs...)
	record.SetTimestamp(time.Now())
	logger.Emit(context.Background(), record)
}
