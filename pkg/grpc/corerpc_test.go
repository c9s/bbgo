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
)

const testToken = "test-token"

func startTestServer(t *testing.T, environ *bbgo.Environment) pb.CoreRPCServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(CoreRPCInterceptor(testToken)))
	pb.RegisterCoreRPCServiceServer(grpcServer, &CoreRPCService{Environ: environ})

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

	return pb.NewCoreRPCServiceClient(conn)
}

func withAuth(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+testToken)
}

func TestCoreRPCService_RejectsWithoutToken(t *testing.T) {
	environ := bbgo.NewEnvironment()
	client := startTestServer(t, environ)

	_, err := client.ListMethods(context.Background(), &pb.ListMethodsRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCoreRPCService_ListMethods(t *testing.T) {
	environ := bbgo.NewEnvironment()
	require.NoError(t, environ.RPCRegistry().Export("ping", "demo method", false, func() (string, error) {
		return "pong", nil
	}))

	client := startTestServer(t, environ)

	resp, err := client.ListMethods(withAuth(context.Background()), &pb.ListMethodsRequest{})
	require.NoError(t, err)
	require.Nil(t, resp.Error)
	require.Len(t, resp.Methods, 1)
	assert.Equal(t, "ping", resp.Methods[0].Name)
	assert.False(t, resp.Methods[0].Mutating)
}

func TestCoreRPCService_Invoke_Success(t *testing.T) {
	environ := bbgo.NewEnvironment()
	require.NoError(t, environ.RPCRegistry().Export("ping", "demo method", false, func() (string, error) {
		return "pong", nil
	}))

	client := startTestServer(t, environ)

	resp, err := client.Invoke(withAuth(context.Background()), &pb.InvokeRequest{
		Method:     "ping",
		ParamsJson: `[]`,
	})
	require.NoError(t, err)
	require.Nil(t, resp.Error)
	assert.Equal(t, `"pong"`, resp.ResultJson)
}

func TestCoreRPCService_Invoke_UnknownMethod(t *testing.T) {
	environ := bbgo.NewEnvironment()
	client := startTestServer(t, environ)

	resp, err := client.Invoke(withAuth(context.Background()), &pb.InvokeRequest{
		Method:     "does-not-exist",
		ParamsJson: `[]`,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
}

func TestCoreRPCService_Invoke_DeadlineExceeded(t *testing.T) {
	environ := bbgo.NewEnvironment()

	unblock := make(chan struct{})
	t.Cleanup(func() { close(unblock) })
	require.NoError(t, environ.RPCRegistry().Export("stall", "never returns in time", false, func() error {
		<-unblock
		return nil
	}))

	client := startTestServer(t, environ)

	ctx, cancel := context.WithTimeout(withAuth(context.Background()), 100*time.Millisecond)
	defer cancel()

	_, err := client.Invoke(ctx, &pb.InvokeRequest{
		Method:     "stall",
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
