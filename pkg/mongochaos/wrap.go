package mongochaos

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/CemAkan/pastaay/pkg/tracing"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type WrappedClient struct {
	*mongo.Client
	mgr *config.Manager
}

func WrapClient(c *mongo.Client, mgr *config.Manager) *WrappedClient {
	return &WrappedClient{Client: c, mgr: mgr}
}

func (w *WrappedClient) Database(name string, opts ...interface{}) *Database {
	if w == nil || w.Client == nil {
		return &Database{name: name}
	}
	return &Database{db: w.Client.Database(name), mgr: w.mgr, name: name}
}

type Database struct {
	db   *mongo.Database
	mgr  *config.Manager
	name string
}

func (d *Database) RunCommand(ctx context.Context, runCommand interface{}, opts ...interface{}) *mongo.SingleResult {
	if err := d.applyChaos(ctx, "runCommand"); err != nil {
		return mongo.NewSingleResultFromDocument(nil, err, nil)
	}
	if d.db == nil {
		return mongo.NewSingleResultFromDocument(nil, errors.New("pastaay: mongo database unavailable"), nil)
	}
	return d.db.RunCommand(ctx, runCommand)
}

func (d *Database) applyChaos(ctx context.Context, op string) error {
	if d == nil || d.mgr == nil {
		return nil
	}
	if d.mgr.IsCommandIgnored("mongo", op) {
		return nil
	}
	for _, p := range d.mgr.GetActivePolicies("mongo") {
		if !(strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, op)) {
			continue
		}
		latencyHit := p.LatencyChance > 0 && rand.Float64() < p.LatencyChance
		errorHit := p.ErrorChance > 0 && rand.Float64() < p.ErrorChance
		if latencyHit && errorHit {
			latencyHit = false
		}
		if latencyHit {
			metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "latency").Inc()
			spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.mongo.latency", op, "latency")
			telemetry.EmitInfo("mongo", "Mongo Latency Injected", map[string]interface{}{
				"duration": p.LatencyDuration.String(), "target": op,
			}, span)
			timer := time.NewTimer(p.LatencyDuration)
			select {
			case <-timer.C:
				timer.Stop()
				span.End()
			case <-spanCtx.Done():
				timer.Stop()
				span.End()
				return spanCtx.Err()
			}
		}
		if errorHit {
			metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "error").Inc()
			_, span := tracing.StartChaosSpan(ctx, "pastaay.mongo.error", op, "error")
			defer span.End()
			msg := p.ErrorBody
			if msg == "" {
				msg = "pastaay: synthetic mongo fault"
			}
			telemetry.EmitError("mongo", op, "Mongo Fault Injected", msg, span)
			return errors.New(msg)
		}
	}
	return nil
}
