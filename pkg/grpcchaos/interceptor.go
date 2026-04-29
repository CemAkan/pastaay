package grpcchaos

import (
	"context"
	"log"
	"math/rand"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

// UnaryInterceptor evaluates gRPC policies for point-to-point calls.
func UnaryInterceptor(mgr *config.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := applyGrpcChaos(ctx, mgr, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor evaluates gRPC policies for streaming RPCs.
func StreamInterceptor(mgr *config.Manager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := applyGrpcChaos(ss.Context(), mgr, info.FullMethod); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func applyGrpcChaos(ctx context.Context, mgr *config.Manager, method string) error {
	policies := mgr.GetActivePolicies("grpc")

	for _, p := range policies {
		if p.Target == method && matchMetadata(ctx, p.MatchHeaders) {
			// Latency Injection
			if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
				log.Printf("[Pastaay-gRPC] Latency: delaying %s by %v", method, p.LatencyDuration)
				metrics.InjectedFaultsTotal.WithLabelValues(method, "latency").Inc()
				time.Sleep(p.LatencyDuration)
			}

			// Error Injection
			if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
				log.Printf("[Pastaay-gRPC] Error: injecting Unavailable to %s", method)
				metrics.InjectedFaultsTotal.WithLabelValues(method, "error").Inc()
				return status.Error(codes.Unavailable, "Pastaay Chaos Injected")
			}
			break
		}
	}
	return nil
}

func matchMetadata(ctx context.Context, required map[string]string) bool {
	if len(required) == 0 {
		return true
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	for k, v := range required {
		vals := md.Get(k)
		if len(vals) == 0 || vals[0] != v {
			return false
		}
	}
	return true
}
