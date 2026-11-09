package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/rpcexport"
)

const testToken = "test-token"

// testExportingStrategy is a minimal CrossExchangeStrategy that also
// implements bbgo.RPCExporter, and services its own inbox from a
// goroutine started in CrossRun — mirroring how a real strategy is
// expected to drain reg.Requests() from its own Run()/CrossRun() loop.
type testExportingStrategy struct {
	reg *rpcexport.Registry
}

func (s *testExportingStrategy) ID() string { return "test-strategy" }

func (s *testExportingStrategy) InstanceID() string { return "test-strategy:instance-1" }

func (s *testExportingStrategy) ExportRPC(reg *rpcexport.Registry) {
	s.reg = reg
	reg.Export("echo", "echoes back its input", false, func(msg string) (string, error) {
		return msg, nil
	}, "msg")
	reg.Export("explode", "always panics", true, func() error {
		panic("boom")
	})
}

func (s *testExportingStrategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, _ map[string]*bbgo.ExchangeSession) error {
	go func() {
		for {
			select {
			case call := <-s.reg.Requests():
				s.reg.Dispatch(call)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// stalledExportingStrategy exports a method but never drains its inbox, to
// exercise the Invoke deadline path.
type stalledExportingStrategy struct {
	reg *rpcexport.Registry
}

func (s *stalledExportingStrategy) ID() string { return "stalled-strategy" }

func (s *stalledExportingStrategy) InstanceID() string { return "stalled-strategy:instance-1" }

func (s *stalledExportingStrategy) ExportRPC(reg *rpcexport.Registry) {
	s.reg = reg
	reg.Export("noop", "never serviced", false, func() error { return nil })
}

func (s *stalledExportingStrategy) CrossRun(_ context.Context, _ bbgo.OrderExecutionRouter, _ map[string]*bbgo.ExchangeSession) error {
	return nil
}

func startTestServer(t *testing.T, trader *bbgo.Trader) pb.StrategyRPCServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(StrategyRPCInterceptor(testToken)))
	pb.RegisterStrategyRPCServiceServer(grpcServer, &StrategyRPCService{Trader: trader})

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewStrategyRPCServiceClient(conn)
}

func withAuth(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+testToken)
}

func setupTrader(t *testing.T, strategies ...bbgo.CrossExchangeStrategy) *bbgo.Trader {
	t.Helper()

	environ := bbgo.NewEnvironment()
	trader := bbgo.NewTrader(environ)
	for _, s := range strategies {
		trader.AttachCrossExchangeStrategy(s)
	}
	require.NoError(t, trader.Initialize(context.Background()))
	return trader
}

func TestStrategyRPCService_RejectsWithoutToken(t *testing.T) {
	trader := setupTrader(t, &testExportingStrategy{})
	client := startTestServer(t, trader)

	_, err := client.ListInstances(context.Background(), &pb.ListInstancesRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestStrategyRPCService_ListInstancesAndMethods(t *testing.T) {
	strategy := &testExportingStrategy{}
	trader := setupTrader(t, strategy)
	client := startTestServer(t, trader)

	instResp, err := client.ListInstances(withAuth(context.Background()), &pb.ListInstancesRequest{})
	require.NoError(t, err)
	require.Len(t, instResp.Instances, 1)
	assert.Equal(t, "test-strategy:instance-1", instResp.Instances[0].InstanceId)
	assert.Equal(t, "test-strategy", instResp.Instances[0].StrategyId)

	methodsResp, err := client.ListMethods(withAuth(context.Background()), &pb.ListMethodsRequest{
		InstanceId: "test-strategy:instance-1",
	})
	require.NoError(t, err)
	require.Nil(t, methodsResp.Error)
	require.Len(t, methodsResp.Methods, 2)
}

func TestStrategyRPCService_Invoke_Success(t *testing.T) {
	strategy := &testExportingStrategy{}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	trader := setupTrader(t, strategy)
	require.NoError(t, strategy.CrossRun(ctx, nil, nil))

	client := startTestServer(t, trader)

	resp, err := client.Invoke(withAuth(context.Background()), &pb.InvokeRequest{
		InstanceId: "test-strategy:instance-1",
		Method:     "echo",
		ParamsJson: `["hello"]`,
	})
	require.NoError(t, err)
	require.Nil(t, resp.Error)
	assert.Equal(t, `"hello"`, resp.ResultJson)
}

func TestStrategyRPCService_Invoke_PanicRecovered(t *testing.T) {
	strategy := &testExportingStrategy{}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	trader := setupTrader(t, strategy)
	require.NoError(t, strategy.CrossRun(ctx, nil, nil))

	client := startTestServer(t, trader)

	resp, err := client.Invoke(withAuth(context.Background()), &pb.InvokeRequest{
		InstanceId: "test-strategy:instance-1",
		Method:     "explode",
		ParamsJson: `[]`,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.ErrorMessage, "boom")
}

func TestStrategyRPCService_Invoke_DeadlineExceeded(t *testing.T) {
	strategy := &stalledExportingStrategy{}
	trader := setupTrader(t, strategy)
	client := startTestServer(t, trader)

	ctx, cancel := context.WithTimeout(withAuth(context.Background()), 200*time.Millisecond)
	defer cancel()

	_, err := client.Invoke(ctx, &pb.InvokeRequest{
		InstanceId: "stalled-strategy:instance-1",
		Method:     "noop",
		ParamsJson: `[]`,
	})
	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

func TestResolveInvokeTimeout_Override(t *testing.T) {
	assert.Equal(t, 250*time.Millisecond, resolveInvokeTimeout("250ms", defaultInvokeTimeout))
}

func TestResolveInvokeTimeout_EmptyFallsBackToDefault(t *testing.T) {
	assert.Equal(t, defaultInvokeTimeout, resolveInvokeTimeout("", defaultInvokeTimeout))
}

func TestResolveInvokeTimeout_InvalidValueFallsBackToDefault(t *testing.T) {
	assert.Equal(t, defaultInvokeTimeout, resolveInvokeTimeout("not-a-duration", defaultInvokeTimeout))
}

func TestStrategyRPCService_Invoke_UnknownInstance(t *testing.T) {
	trader := setupTrader(t)
	client := startTestServer(t, trader)

	resp, err := client.Invoke(withAuth(context.Background()), &pb.InvokeRequest{
		InstanceId: "does-not-exist",
		Method:     "noop",
		ParamsJson: `[]`,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
}
