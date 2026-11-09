package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/rpcexport"
)

// defaultInvokeTimeout bounds how long Invoke waits for the target
// strategy instance to drain and service a call, when the caller didn't
// set a shorter deadline on the context themselves. Overridable at
// startup via the BBGO_STRATEGY_RPC_TIMEOUT env var (e.g. "10s"), see init.
var defaultInvokeTimeout = 5 * time.Second

// strategyRPCTimeoutEnvVar overrides defaultInvokeTimeout when set to a
// value time.ParseDuration accepts, e.g. BBGO_STRATEGY_RPC_TIMEOUT=10s.
const strategyRPCTimeoutEnvVar = "BBGO_STRATEGY_RPC_TIMEOUT"

func init() {
	defaultInvokeTimeout = resolveInvokeTimeout(os.Getenv(strategyRPCTimeoutEnvVar), defaultInvokeTimeout)
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
		log.WithError(err).Warnf("invalid %s=%q, falling back to %s", strategyRPCTimeoutEnvVar, raw, fallback)
		return fallback
	}

	return d
}

// StrategyRPCService implements the gRPC service that lets a caller
// discover and invoke methods a strategy instance has exported via
// bbgo.RPCExporter. Every method here is expected to run behind the token
// auth interceptor registered in Server.ListenAndServe.
type StrategyRPCService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedStrategyRPCServiceServer
}

func (s *StrategyRPCService) ListInstances(_ context.Context, _ *pb.ListInstancesRequest) (*pb.ListInstancesResponse, error) {
	instances := s.Trader.ListRPCInstances()

	resp := &pb.ListInstancesResponse{}
	for _, inst := range instances {
		resp.Instances = append(resp.Instances, &pb.StrategyInstance{
			InstanceId: inst.InstanceID,
			StrategyId: inst.StrategyID,
		})
	}

	return resp, nil
}

func (s *StrategyRPCService) ListMethods(_ context.Context, request *pb.ListMethodsRequest) (*pb.ListMethodsResponse, error) {
	registry, ok := s.Trader.RPCRegistry(request.InstanceId)
	if !ok {
		return &pb.ListMethodsResponse{
			Error: &pb.Error{ErrorMessage: fmt.Sprintf("instance %q not found", request.InstanceId)},
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

func (s *StrategyRPCService) Invoke(ctx context.Context, request *pb.InvokeRequest) (*pb.InvokeResponse, error) {
	registry, ok := s.Trader.RPCRegistry(request.InstanceId)
	if !ok {
		return &pb.InvokeResponse{
			Error: &pb.Error{ErrorMessage: fmt.Sprintf("instance %q not found", request.InstanceId)},
		}, nil
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultInvokeTimeout)
		defer cancel()
	}

	resultCh, err := registry.Enqueue(ctx, request.Method, json.RawMessage(request.ParamsJson))
	if err != nil {
		if errors.Is(err, rpcexport.ErrInboxFull) {
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		}
		// method-not-found and similar lookup failures are business-logic
		// errors, reported via the Error field like every other response.
		return &pb.InvokeResponse{Error: &pb.Error{ErrorMessage: err.Error()}}, nil
	}

	select {
	case res := <-resultCh:
		resp := &pb.InvokeResponse{ResultJson: string(res.Data)}
		if res.Err != nil {
			resp.Error = &pb.Error{ErrorMessage: res.Err.Error()}
		}
		return resp, nil

	case <-ctx.Done():
		return nil, status.Error(codes.DeadlineExceeded, fmt.Sprintf("instance %q did not service method %q in time", request.InstanceId, request.Method))
	}
}
