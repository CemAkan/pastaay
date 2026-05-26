package grpcchaos

import (
	"context"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/CemAkan/pastaay/pkg/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/tracing"
)

type PolicyDecision struct {
	Latency time.Duration
	Err     error
	Hash    uint64
}

type chaosServerStream struct {
	grpc.ServerStream
	method          string
	cfgManager      *config.Manager
	mu              sync.Mutex
	decidedPolicies map[string]PolicyDecision
	isIgnored       bool
	incomingMD      metadata.MD
}

func (s *chaosServerStream) SendMsg(m interface{}) error {
	return s.evaluate(s.Context(), func() error { return s.ServerStream.SendMsg(m) })
}

func (s *chaosServerStream) RecvMsg(m interface{}) error {
	return s.evaluate(s.Context(), func() error { return s.ServerStream.RecvMsg(m) })
}

func (s *chaosServerStream) evaluate(ctx context.Context, next func() error) error {
	if s.isIgnored {
		return next()
	}

	policies := s.cfgManager.GetActivePolicies("grpc")
	for _, p := range policies {
		if (strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, s.method)) &&
			matchMetadataPure(s.incomingMD, p.MatchHeaders) {

			var decision PolicyDecision
			isStreamMode := strings.EqualFold(p.StreamRollMode, "stream")

			if isStreamMode && s.decidedPolicies != nil {
				s.mu.Lock()
				d, decided := s.decidedPolicies[p.Name]

				if decided && d.Hash == p.PolicyHash {
					s.mu.Unlock()
					if d.Latency > 0 {

						metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "latency").Inc()
						spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.grpc.latency", s.method, "latency")
						if err := waitContext(spanCtx, d.Latency); err != nil {
							span.End()
							return err
						}
						span.End()
					}
					if d.Err != nil {

						metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "error").Inc()
						_, span := tracing.StartChaosSpan(ctx, "pastaay.grpc.error", s.method, "error")
						span.End()
						return d.Err
					}
					continue
				}

				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					decision.Latency = p.LatencyDuration
				}
				if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
					decision.Err = generateError(p)
				}
				decision.Hash = p.PolicyHash

				s.decidedPolicies[p.Name] = decision
				s.mu.Unlock()
			} else {
				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					decision.Latency = p.LatencyDuration
				}
				if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
					decision.Err = generateError(p)
				}
			}

			metricTag := p.MetricTag

			if decision.Latency > 0 {
				metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
				spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.grpc.latency", s.method, "latency")
				if err := waitContext(spanCtx, decision.Latency); err != nil {
					span.End()
					return err
				}
				span.End()
			}
			if decision.Err != nil {
				metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
				_, span := tracing.StartChaosSpan(ctx, "pastaay.grpc.error", s.method, "error")

				telemetry.EmitError("grpc", s.method, "gRPC Fault Injected", decision.Err.Error(), span)

				span.End()
				return decision.Err
			}
		}
	}
	return next()
}

func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		timer.Stop()
		return nil
	case <-ctx.Done():
		timer.Stop()
		return status.FromContextError(ctx.Err()).Err()
	}
}

func matchMetadataPure(md metadata.MD, required map[string]string) bool {
	if len(required) == 0 {
		return true
	}
	if md == nil {
		return false
	}
	for reqK, reqV := range required {
		found := false
		for mdK, vals := range md {
			if strings.EqualFold(mdK, reqK) {
				for _, v := range vals {

					if strings.EqualFold(v, reqV) {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func generateError(p config.Policy) error {
	msg := p.ErrorBody
	if msg == "" {
		msg = "Pastaay Chaos Injected"
	}
	grpcCode := codes.Unavailable
	if p.ErrorCode > 0 {
		grpcCode = codes.Code(p.ErrorCode)
	}
	return status.Error(grpcCode, msg)
}

func UnaryInterceptor(mgr *config.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if mgr.IsCommandIgnored("grpc", info.FullMethod) {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		stream := &chaosServerStream{method: info.FullMethod, cfgManager: mgr, incomingMD: md}
		err := stream.evaluate(ctx, func() error { return nil })
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func StreamInterceptor(mgr *config.Manager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, _ := metadata.FromIncomingContext(ss.Context())
		wrapper := &chaosServerStream{
			ServerStream:    ss,
			method:          info.FullMethod,
			cfgManager:      mgr,
			decidedPolicies: make(map[string]PolicyDecision),
			isIgnored:       mgr.IsCommandIgnored("grpc", info.FullMethod),
			incomingMD:      md,
		}
		return handler(srv, wrapper)
	}
}
