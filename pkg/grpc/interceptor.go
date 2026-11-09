package grpc

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// strategyRPCMethodPrefix is the gRPC full-method prefix for every RPC on
// StrategyRPCService. Only calls under this prefix are gated/audited;
// MarketDataService/TradingService/UserDataService are unaffected.
const strategyRPCMethodPrefix = "/" + "bbgo.StrategyRPCService" + "/"

// StrategyRPCInterceptor enforces a shared bearer token on every call to
// StrategyRPCService (list and invoke alike) and logs every call that
// reaches it, successful or not — this is the audit log for strategy RPC
// invocations.
func StrategyRPCInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !strings.HasPrefix(info.FullMethod, strategyRPCMethodPrefix) {
			return handler(ctx, req)
		}

		if err := checkToken(ctx, token); err != nil {
			log.WithField("method", info.FullMethod).Warn("strategy rpc: rejected unauthenticated call")
			return nil, err
		}

		start := time.Now()
		resp, err := handler(ctx, req)

		entry := log.WithFields(log.Fields{
			"method":   info.FullMethod,
			"duration": time.Since(start),
		})
		if err != nil {
			entry.WithError(err).Warn("strategy rpc: call failed")
		} else {
			entry.Info("strategy rpc: call completed")
		}

		return resp, err
	}
}

func checkToken(ctx context.Context, token string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing authorization metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization token")
	}

	given := strings.TrimPrefix(values[0], "Bearer ")
	if subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid authorization token")
	}

	return nil
}
