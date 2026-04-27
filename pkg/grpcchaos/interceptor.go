package grpcchaos

import (
	"context"

	"github.com/CemAkan/pastaay/pkg/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryInterceptor returns a gRPC server interceptor that applies chaos policies.
func UnaryInterceptor(cfgManager *config.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		// TODO: Chaos logic will be implemented here

		// Proceed to the actual RPC handler normally
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
