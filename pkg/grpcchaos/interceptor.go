package grpcchaos

import (
	"context"

	"google.golang.org/grpc"

	"github.com/CemAkan/pastaay/pkg/config"
)

// UnaryInterceptor returns a gRPC server interceptor that applies chaos policies.
func UnaryInterceptor(cfgManager *config.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		// TODO: Chaos logic will be implemented here

		// Proceed to the actual RPC handler normally
		return handler(ctx, req)
	}
}
