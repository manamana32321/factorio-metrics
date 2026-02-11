package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gorcon/rcon"
	"go.opentelemetry.io/otel/metric"
)

// FactorioStats represents the JSON output from the Lua collection script.
type FactorioStats struct {
	Tick             int64              `json:"tick"`
	Players          int64              `json:"players"`
	Evolution        float64            `json:"evolution"`
	ItemProduction   map[string]float64 `json:"item_production"`
	ItemConsumption  map[string]float64 `json:"item_consumption"`
	FluidProduction  map[string]float64 `json:"fluid_production"`
	FluidConsumption map[string]float64 `json:"fluid_consumption"`
	KillCounts       map[string]float64 `json:"kill_counts"`
	EntityBuilt      map[string]float64 `json:"entity_built"`
	PowerProduction  map[string]float64 `json:"power_production"`
	PowerConsumption map[string]float64 `json:"power_consumption"`
	RocketsLaunched  int64              `json:"rockets_launched"`
	Research         *string            `json:"research"`
	ResearchProgress float64            `json:"research_progress"`
}

// Collector collects Factorio metrics via RCON and exports them as OTel gauges.
type Collector struct {
	addr     string
	password string
	lua      string

	// OTel instruments
	players          metric.Int64Gauge
	evolution        metric.Float64Gauge
	tick             metric.Int64Gauge
	rocketsLaunched  metric.Int64Gauge
	researchProgress metric.Float64Gauge
	itemProduction   metric.Float64Gauge
	itemConsumption  metric.Float64Gauge
	fluidProduction  metric.Float64Gauge
	fluidConsumption metric.Float64Gauge
	powerProduction  metric.Float64Gauge
	powerConsumption metric.Float64Gauge
	killCount        metric.Float64Gauge
	entityBuilt      metric.Float64Gauge
}

func NewCollector(host, port, password, luaScript string, mp *metric.MeterProvider) (*Collector, error) {
	meter := mp.Meter("factorio")
	c := &Collector{
		addr:     fmt.Sprintf("%s:%s", host, port),
		password: password,
		lua:      luaScript,
	}

	var err error
	c.players, err = meter.Int64Gauge("factorio_players")
	if err != nil {
		return nil, err
	}
	c.evolution, err = meter.Float64Gauge("factorio_evolution")
	if err != nil {
		return nil, err
	}
	c.tick, err = meter.Int64Gauge("factorio_tick")
	if err != nil {
		return nil, err
	}
	c.rocketsLaunched, err = meter.Int64Gauge("factorio_rockets_launched")
	if err != nil {
		return nil, err
	}
	c.researchProgress, err = meter.Float64Gauge("factorio_research_progress")
	if err != nil {
		return nil, err
	}
	c.itemProduction, err = meter.Float64Gauge("factorio_item_production")
	if err != nil {
		return nil, err
	}
	c.itemConsumption, err = meter.Float64Gauge("factorio_item_consumption")
	if err != nil {
		return nil, err
	}
	c.fluidProduction, err = meter.Float64Gauge("factorio_fluid_production")
	if err != nil {
		return nil, err
	}
	c.fluidConsumption, err = meter.Float64Gauge("factorio_fluid_consumption")
	if err != nil {
		return nil, err
	}
	c.powerProduction, err = meter.Float64Gauge("factorio_power_production")
	if err != nil {
		return nil, err
	}
	c.powerConsumption, err = meter.Float64Gauge("factorio_power_consumption")
	if err != nil {
		return nil, err
	}
	c.killCount, err = meter.Float64Gauge("factorio_kill_count")
	if err != nil {
		return nil, err
	}
	c.entityBuilt, err = meter.Float64Gauge("factorio_entity_built")
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Collector) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Collect once immediately
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	conn, err := rcon.Dial(c.addr, c.password)
	if err != nil {
		log.Printf("rcon dial error: %v", err)
		return
	}
	defer conn.Close()

	// Execute Lua via /sc (silent command)
	resp, err := conn.Execute("/sc " + c.lua)
	if err != nil {
		log.Printf("rcon execute error: %v", err)
		return
	}

	resp = strings.TrimSpace(resp)
	if resp == "" {
		log.Println("empty response from rcon")
		return
	}

	var stats FactorioStats
	if err := json.Unmarshal([]byte(resp), &stats); err != nil {
		log.Printf("json parse error: %v (response: %.200s)", err, resp)
		return
	}

	c.record(ctx, &stats)
}

func (c *Collector) record(ctx context.Context, s *FactorioStats) {
	c.players.Record(ctx, s.Players)
	c.evolution.Record(ctx, s.Evolution)
	c.tick.Record(ctx, s.Tick)
	c.rocketsLaunched.Record(ctx, s.RocketsLaunched)
	c.researchProgress.Record(ctx, s.ResearchProgress)

	for name, val := range s.ItemProduction {
		c.itemProduction.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.ItemConsumption {
		c.itemConsumption.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.FluidProduction {
		c.fluidProduction.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.FluidConsumption {
		c.fluidConsumption.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.PowerProduction {
		c.powerProduction.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.PowerConsumption {
		c.powerConsumption.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.KillCounts {
		c.killCount.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
	for name, val := range s.EntityBuilt {
		c.entityBuilt.Record(ctx, val, metric.WithAttributes(nameAttribute(name)))
	}
}
