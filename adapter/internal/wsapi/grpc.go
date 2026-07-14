package wsapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	ws "github.com/yandex-cloud/go-genproto/yandex/cloud/serverless/apigateway/websocket/v1"
)

const grpcEndpoint = "apigateway-connections.api.cloud.yandex.net:443"

const grpcCallTimeout = 5 * time.Second

type grpcClient struct {
	once   sync.Once
	client ws.ConnectionServiceClient
	conn   *grpc.ClientConn
	err    error
}

func (g *grpcClient) init() {
	g.once.Do(func() {
		creds := credentials.NewTLS(&tls.Config{})
		g.conn, g.err = grpc.NewClient(grpcEndpoint, grpc.WithTransportCredentials(creds))
		if g.err != nil {
			g.err = fmt.Errorf("grpc dial: %w", g.err)
			return
		}
		g.client = ws.NewConnectionServiceClient(g.conn)
		log.Println("gRPC WS API client initialized:", grpcEndpoint)
	})
}

func (g *grpcClient) callCtx(iamToken string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), grpcCallTimeout)
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + iamToken,
	})
	return metadata.NewOutgoingContext(ctx, md), cancel
}

func (g *grpcClient) Send(connectionId string, data []byte, dataType string, iamToken string) error {
	g.init()
	if g.err != nil {
		return g.err
	}

	t := ws.SendToConnectionRequest_BINARY
	if dataType == "TEXT" {
		t = ws.SendToConnectionRequest_TEXT
	}

	ctx, cancel := g.callCtx(iamToken)
	defer cancel()
	_, err := g.client.Send(ctx, &ws.SendToConnectionRequest{
		ConnectionId: connectionId,
		Data:         data,
		Type:         t,
	})
	if err != nil {
		log.Println("wsapi.Send gRPC failed:", connectionId, err)
		return err
	}
	return nil
}

func (g *grpcClient) Disconnect(connectionId string, iamToken string) error {
	g.init()
	if g.err != nil {
		return g.err
	}

	ctx, cancel := g.callCtx(iamToken)
	defer cancel()
	_, err := g.client.Disconnect(ctx, &ws.DisconnectRequest{
		ConnectionId: connectionId,
	})
	if err != nil {
		log.Println("wsapi.Disconnect gRPC failed:", connectionId, err)
	}
	return err
}
