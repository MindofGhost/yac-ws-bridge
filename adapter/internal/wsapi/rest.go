package wsapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const restAPIBase = "https://apigateway-connections.api.cloud.yandex.net/apigateways/websocket/v1/connections"

const restCallTimeout = 5 * time.Second

type restClient struct{}

func (r *restClient) Send(connectionId string, data []byte, dataType string, iamToken string) error {
	ctx, cancel := context.WithTimeout(context.Background(), restCallTimeout)
	defer cancel()

	b64 := base64.StdEncoding.EncodeToString(data)
	body, _ := json.Marshal(map[string]string{
		"data": b64,
		"type": dataType,
	})

	url := fmt.Sprintf("%s/%s:send", restAPIBase, connectionId)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+iamToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("wsSend: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		log.Printf("wsapi.Send REST failed: status=%d connId=%s body=%s", resp.StatusCode, connectionId, string(respBody))
		return &StatusError{StatusCode: resp.StatusCode, Operation: "wsSend", ConnectionID: connectionId}
	}
	return nil
}

func (r *restClient) Disconnect(connectionId string, iamToken string) error {
	ctx, cancel := context.WithTimeout(context.Background(), restCallTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/%s:disconnect", restAPIBase, connectionId)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+iamToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return &StatusError{StatusCode: resp.StatusCode, Operation: "wsDisconnect", ConnectionID: connectionId}
	}
	return nil
}
