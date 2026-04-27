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

// UnaryInterceptor returns a gRPC server interceptor that applies chaos policies.
func UnaryInterceptor(cfgManager *config.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		currentConfig := cfgManager.Get()

		var activePolicy *config.Policy
		for _, policy := range currentConfig.Policies {
			// info.FullMethod contains the gRPC route, e.g., "/service.v1.MyService/MyMethod"
			if policy.Type == "grpc" && policy.Target == info.FullMethod {
				if matchMetadata(ctx, policy.MatchHeaders) {
					p := policy
					activePolicy = &p
					break
				}
			}
		}

		if activePolicy != nil {
			// Latency Injection
			if activePolicy.LatencyChance > 0 && rand.Float64() < activePolicy.LatencyChance {
				log.Printf("Pastaay gRPC: Injecting %v latency to %s", activePolicy.LatencyDuration, info.FullMethod)
				metrics.InjectedFaultsTotal.WithLabelValues(info.FullMethod, "latency").Inc()
				time.Sleep(activePolicy.LatencyDuration)
			}

			// Error Injection
			if activePolicy.ErrorChance > 0 && rand.Float64() < activePolicy.ErrorChance {
				log.Printf("Pastaay gRPC: Injecting Unavailable error to %s", info.FullMethod)
				metrics.InjectedFaultsTotal.WithLabelValues(info.FullMethod, "error").Inc()
				return nil, status.Error(codes.Unavailable, "Pastaay Chaos: Synthetic Fault Injected")
			}
		}

		return handler(ctx, req)
	}
}

// matchMetadata verifies if the incoming gRPC context contains the required targeting headers.
func matchMetadata(ctx context.Context, required map[string]string) bool {
	if len(required) == 0 {
		return true
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	for key, expectedValue := range required {
		values := md.Get(key)
		if len(values) == 0 || values[0] != expectedValue {
			return false
		}
	}

	return true
}

// StreamInterceptor returns a gRPC server interceptor for streaming RPCs.
func StreamInterceptor(cfgManager *config.Manager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		currentConfig := cfgManager.Get()

		var activePolicy *config.Policy
		for _, policy := range currentConfig.Policies {
			if policy.Type == "grpc" && policy.Target == info.FullMethod {
				// For streams, the context is fetched from the ServerStream interface
				if matchMetadata(ss.Context(), policy.MatchHeaders) {
					p := policy
					activePolicy = &p
					break
				}
			}
		}

		if activePolicy != nil {
			// Latency Injection
			if activePolicy.LatencyChance > 0 && rand.Float64() < activePolicy.LatencyChance {
				log.Printf("Pastaay gRPC Stream: Injecting %v latency to %s", activePolicy.LatencyDuration, info.FullMethod)
				metrics.InjectedFaultsTotal.WithLabelValues(info.FullMethod, "latency").Inc()
				time.Sleep(activePolicy.LatencyDuration)
			}

			// Error Injection
			if activePolicy.ErrorChance > 0 && rand.Float64() < activePolicy.ErrorChance {
				log.Printf("Pastaay gRPC Stream: Injecting Unavailable error to %s", info.FullMethod)
				metrics.InjectedFaultsTotal.WithLabelValues(info.FullMethod, "error").Inc()
				return status.Error(codes.Unavailable, "Pastaay Chaos: Synthetic Stream Fault Injected")
			}
		}

		return handler(srv, ss)
	}
}
