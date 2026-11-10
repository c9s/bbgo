package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
)

// defaultInvokeTimeout bounds how long Invoke waits for a method to
// return, when the caller didn't set a shorter deadline on the context
// themselves. Overridable at startup via the BBGO_CORE_GRPC_TIMEOUT env
// var (e.g. "10s"), see init.
var defaultInvokeTimeout = 5 * time.Second

// coreGRPCTimeoutEnvVar overrides defaultInvokeTimeout when set to a
// value time.ParseDuration accepts, e.g. BBGO_CORE_GRPC_TIMEOUT=10s.
const coreGRPCTimeoutEnvVar = "BBGO_CORE_GRPC_TIMEOUT"

func init() {
	defaultInvokeTimeout = resolveInvokeTimeout(os.Getenv(coreGRPCTimeoutEnvVar), defaultInvokeTimeout)
}

// resolveInvokeTimeout parses raw as a duration, returning fallback when
// raw is empty or not a valid duration. Factored out of init so the
// parsing/fallback logic can be unit tested directly.
func resolveInvokeTimeout(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		log.WithError(err).Warnf("invalid %s=%q, falling back to %s", coreGRPCTimeoutEnvVar, raw, fallback)
		return fallback
	}

	return d
}

// CoreRPCService implements the gRPC service that lets a caller discover
// and invoke methods bbgo has exported via Environment.RPCRegistry().
// Every method here is expected to run behind the token auth
// interceptor registered in Server.ListenAndServe.
type CoreRPCService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedCoreRPCServiceServer
}

func (s *CoreRPCService) ListMethods(_ context.Context, _ *pb.ListMethodsRequest) (*pb.ListMethodsResponse, error) {
	registry := s.Environ.RPCRegistry()
	if registry == nil {
		return &pb.ListMethodsResponse{
			Error: &pb.Error{ErrorMessage: "core rpc registry is not initialized"},
		}, nil
	}

	resp := &pb.ListMethodsResponse{}
	for _, m := range registry.Methods() {
		info := &pb.MethodInfo{
			Name:        m.Name,
			Description: m.Description,
			Mutating:    m.Mutating,
		}
		for _, p := range m.Params {
			info.Params = append(info.Params, &pb.ParamSchema{Name: p.Name, Type: p.Type})
		}
		resp.Methods = append(resp.Methods, info)
	}

	return resp, nil
}

func (s *CoreRPCService) Invoke(ctx context.Context, request *pb.InvokeRequest) (*pb.InvokeResponse, error) {
	registry := s.Environ.RPCRegistry()
	if registry == nil {
		return &pb.InvokeResponse{
			Error: &pb.Error{ErrorMessage: "core rpc registry is not initialized"},
		}, nil
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultInvokeTimeout)
		defer cancel()
	}

	data, err := registry.Invoke(ctx, request.Method, json.RawMessage(request.ParamsJson))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		}
		// method-not-found and similar lookup failures are business-logic
		// errors, reported via the Error field like every other response.
		return &pb.InvokeResponse{Error: &pb.Error{ErrorMessage: err.Error()}}, nil
	}

	return &pb.InvokeResponse{ResultJson: string(data)}, nil
}
