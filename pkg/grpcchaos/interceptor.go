package grpcchaos

import (
	"context"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

type chaosServerStream struct {
	grpc.ServerStream
	method     string
	cfgManager *config.Manager
}

func (s *chaosServerStream) SendMsg(m interface{}) error {
	if err := applyGrpcChaos(s.Context(), s.cfgManager, s.method); err != nil {
		return err
	}
	return s.ServerStream.SendMsg(m)
}

func (s *chaosServerStream) RecvMsg(m interface{}) error {
	if err := applyGrpcChaos(s.Context(), s.cfgManager, s.method); err != nil {
		return err
	}
	return s.ServerStream.RecvMsg(m)
}

func UnaryInterceptor(mgr *config.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := applyGrpcChaos(ctx, mgr, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func StreamInterceptor(mgr *config.Manager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapper := &chaosServerStream{
			ServerStream: ss,
			method:       info.FullMethod,
			cfgManager:   mgr,
		}
		return handler(srv, wrapper)
	}
}

func applyGrpcChaos(ctx context.Context, mgr *config.Manager, method string) error {
	policies := mgr.GetActivePolicies("grpc")

	for _, p := range policies {
		// Case-insensitive (EqualFold)
		if (strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, method)) && matchMetadata(ctx, p.MatchHeaders) {
			if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
				log.Printf("[Pastaay-gRPC] Latency: delaying %s by %v", method, p.LatencyDuration)
				metrics.InjectedFaultsTotal.WithLabelValues(method, "latency").Inc()

				timer := time.NewTimer(p.LatencyDuration)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				}
				timer.Stop()
			}

			if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
				msg := p.ErrorBody
				if msg == "" {
					msg = "Pastaay Chaos Injected"
				}

				grpcCode := codes.Unavailable
				if p.ErrorCode > 0 {
					grpcCode = codes.Code(p.ErrorCode)
				}
				metrics.InjectedFaultsTotal.WithLabelValues(method, "error").Inc()
				return status.Error(grpcCode, msg)
			}
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
		vals := md.Get(strings.ToLower(k))
		if len(vals) == 0 || vals[0] != v {
			return false
		}
	}
	return true
}
