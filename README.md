# Bridge to Freedom

A proxy tunnel through Yandex Cloud.
This branch contains the single-adapter variant: it proxies WebSocket connections (VLESS with WebSocket transport or XMPP over WebSockets, for example), requires no client modifications or additional software on the client device, and only requires you to use the serverless function address as the WebSocket connection URL.
The serverless function forwards data in both directions.

## Changes

- **2026-07-14** — Improved connection stability: added network-call and HELLO timeouts, safe handling of errors when sending data to clients, upstream publication only after HELLO with background refresh, and idempotent handling of repeated `CLIENT_CONNECTED` events.
- **2026-06-15** — Recommended settings for fewer disconnects: use `reorder.c2tDelayMs: 100` and `c2tMaxDelayMs: 350` in the adapter (if disconnects remain frequent, increase them by 2–3 times), and set the cloud function's `concurrency` to `4` instead of 16 (fewer parallel invocations mean less packet reordering). See the Recommendations section and the configuration parameters for details.
- **2026-06-15** — Fixed frame sorting direction: Yandex API Gateway message IDs turned out to be ordered in **reverse chronological order** (a larger ID means an earlier message, while the newest message has the smallest ID). The adapter sorted them in ascending order, reversing frames that had already arrived in the correct order and incorrectly dropping connections because of `isLateSeq`. This explains why buffering “only made things worse” and why longer timeouts did not help. Sorting and comparisons now use descending order, configurable through `reorder.seqOrder`.
- **2026-06-15** — Additional stability improvements: the serverless function no longer silently loses DATA frames (if delivery fails, it retries via POST; if that also fails, the connection is forcibly reset so the client reconnects instead of continuing with a corrupted TLS stream); removed a blocking delay on cold instances that caused packets to arrive out of order; and added an adapter counter for resets caused by late frames to aid diagnostics.

## How It Actually Works

The main idea is to use the tunnel together with VLESS over WebSocket.
It works poorly.
Because of how Yandex Serverless Functions and API Gateway operate, packets sometimes arrive in the wrong order (the server reorders messages while receiving them) or are occasionally lost.

You can surf the internet, but TLS handshakes are rather slow. Once a connection is established, throughput can be decent (4–5 Mbps), but connections periodically drop. After a while, sites whose resources all come from one server and cache well can even be reasonably comfortable to use.

Mobile apps vary. Reddit, for example, is quite usable, though you sometimes have to tap Refresh when a post or its comments fail to load. Telegram unexpectedly works fairly well: it often falls back to Connecting/Updating, but messages are delivered reliably, and even images and videos can be viewed - its protocol is apparently well optimized for unstable connections. XMPP over WebSocket also works reasonably well with some clients (such as a patched Conversations). With enough patience, you can even listen to Spotify.

Recommendations for VLESS over WebSocket:
Try `?ed=8000` or something similar in the XRay path. It can sometimes improve performance dramatically, though at other times it makes the connection less stable, so try it both ways. In theory, MUX could help, but nothing works with it. I tried both `mux.cool` from XRay and h2mux/smux/yamux from Sing-box, without success. I suspect they make many small `write()` calls to the socket, each of which becomes a separate WebSocket message and increases the chance of a message being lost or arriving out of order.

## Important

I have no idea how long it will take Yandex to ban you for doing this.

Recommendations:

- Use this only to proxy your most important low-traffic services. For example, tunnel to a SOCKS proxy and configure Telegram to use it at `127.0.0.1`. Do not abuse it by sending large amounts of data. Alternatively, configure the client (v2rayNG and others) to proxy only selected applications so the entire system does not use the proxy.

- Before uploading the serverless function code, run it through any JavaScript obfuscator, perhaps even more than once. Replace all standard paths (`/_upstream`, `/_helper`, and `/_conn-ids`) in the serverless function, adapter, and helper code with random paths of your own.

- Settings that reduce disconnects (chosen empirically and giving the best results at the time of writing):
	- In the adapter (`adapter.config.yaml`), the baseline values are `reorder.c2tDelayMs: 100` and `reorder.c2tMaxDelayMs: 350`. For a more stable connection, use `250` and `1500`; this accommodates serverless invocation delays better but increases latency.
	- Set the cloud function's `concurrency` to `4` instead of 16. Fewer parallel invocations per instance mean fewer sending races and less packet reordering. This is sufficient for such light traffic. With fewer than four instances, however, it becomes severely congested.

## Adapter

Install it on a VPS. Ideally, the VPS should also be in Russia and close to Yandex; from there, proxy onward however and wherever you like.

### Building

Go 1.21+ is required.

```bash
cd adapter
go build -o adapter ./cmd/adapter
```

### Configuration

Create an `adapter.config.yaml` file (an example is included in the repository):

```yaml
bridge:
	url: "wss://<api-gateway-domain>/_upstream"
	authToken: "<shared-secret>"
	reconnect:
		initialDelayMs: 1000
		maxDelayMs: 30000
		backoffMultiplier: 2
	pingIntervalMs: 30000

wakeup:
	listenPort: 3001
	# pathPrefix: "/myprefix"  # optional; must match the path in the cloud function's ADAPTER_URL

target:
	url: "ws://127.0.0.1:9090"

wsApi:
	mode: "grpc"          # "rest" or "grpc"; grpc is faster

reorder:
	c2tDelayMs: 100        # quiet period before flushing the buffer to the target
	c2tMaxDelayMs: 350     # maximum frame hold time; increase by 2–3 times if disconnects are frequent
	seqOrder: "descending" # Yandex message ID order (descending by default)

logging:
	level: "info"
```

For a more stable connection when late frames occur frequently, try `reorder.c2tDelayMs: 250` and `reorder.c2tMaxDelayMs: 1500`. A larger window handles reordering better but increases latency.

| Section | Key | Description |
|---------|-----|-------------|
| **bridge.url** | | WebSocket URL of the API Gateway upstream endpoint |
| **bridge.authToken** | | Shared secret (must match the cloud function's `AUTH_TOKEN` variable) |
| **bridge.reconnect** | | Exponential backoff settings for reconnection |
| **bridge.pingIntervalMs** | | Interval between PING frames sent to prevent idle disconnection |
| **wakeup.listenPort** | | HTTP port for wakeup/fallback endpoints (`0` disables it) |
| **wakeup.pathPrefix** | | Optional URL prefix for all HTTP endpoints (for example, `/myprefix`). Must match the path in the cloud function's `ADAPTER_URL`. |
| **wakeup.tlsCert / tlsKey** | | Optional TLS for the wakeup endpoint |
| **target.url** | | WebSocket URL of the target server (usually `ws://127.0.0.1:<port>`) |
| **wsApi.mode** | | YC management API access method: `rest` or `grpc` |
| **reorder.c2tDelayMs** | | Quiet period (ms) before sending the reordering buffer to the target. Recommended: `100`. |
| **reorder.c2tMaxDelayMs** | | Maximum frame hold time (ms). Recommended: `350`; increase by 2–3 times if disconnects are frequent. |
| **reorder.seqOrder** | | Message ID sorting direction: `descending` (default, matching Yandex's current behavior) or `ascending`. |

### Running

```bash
./adapter adapter.config.yaml
```

The adapter connects to the bridge, authenticates, and starts accepting client traffic. It also starts an HTTP server on the wakeup port with the following endpoints:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/` | POST | Accepts a protocol frame (POST fallback) and initiates upstream reconnection |
| `/upstream-id` | GET | Returns the current upstream connection ID (used by the function on cold start) |
| `/proxy` | POST | Proxies a simple HTTP request to the target server (used when `FORWARD_HTTP` is enabled) |

---

## Deploying the Cloud Function (Yandex Cloud)

### Prerequisites

- A Yandex Cloud account with billing enabled
- The `yc` CLI installed and configured (`yc init`)
- A cloud folder for the project

### 1. Create a Service Account

```bash
yc iam service-account create --name bridge-sa

SA_ID=$(yc iam service-account get bridge-sa --format json | jq -r .id)
FOLDER_ID=$(yc config get folder-id)

# Allow the service account to invoke functions and send WebSocket messages
yc resource-manager folder add-access-binding $FOLDER_ID \
	--role serverless.functions.invoker \
	--subject serviceAccount:$SA_ID

yc resource-manager folder add-access-binding $FOLDER_ID \
	--role api-gateway.websocketBroadcaster \
	--subject serviceAccount:$SA_ID
```

### 2. Create and Deploy the Cloud Function

```bash
cd bridge-cloud
zip -r bridge-function.zip index.js package.json
```

```bash
yc serverless function create --name bridge-fn

FUNCTION_ID=$(yc serverless function get bridge-fn --format json | jq -r .id)
```

Deploy the version:

```bash
yc serverless function version create \
	--function-name bridge-fn \
	--runtime nodejs18 \
	--entrypoint index.handler \
	--memory 128m \
	--execution-timeout 10s \
	--concurrency 4 \
	--source-path bridge-function.zip \
	--service-account-id $SA_ID \
	--environment "AUTH_TOKEN=<your-shared-secret>,ADAPTER_URL=<adapter-wakeup-url>"
```

> `--concurrency 4` (instead of 16) reduces the number of parallel invocations per instance and therefore packet reordering. This is sufficient for light traffic; see the Recommendations section.

| Variable | Required | Description |
|----------|----------|-------------|
| `AUTH_TOKEN` | Yes | Shared secret (the same value as `bridge.authToken` in the adapter configuration) |
| `ADAPTER_URL` | No | HTTP(S) URL of the adapter wakeup endpoint (for example, `https://your-server:3001`). Supports a path prefix (for example, `https://your-server:3001/myprefix`); all HTTP calls to the adapter use this prefix. It must match `wakeup.pathPrefix` in the adapter configuration. Enables POST fallback and cold-start recovery. |
| `FORWARD_HTTP` | No | Set to `true` to proxy regular HTTP requests (GET, POST, and so on) through the adapter to the target server. When enabled, any non-WebSocket request to API Gateway is forwarded to the target server, and its response is returned to the client. |

### 3. Create the API Gateway

Edit `bridge-cloud/spec.yaml` and replace the two placeholders:

- `${FUNCTION_ID}` — function ID from step 2
- `${SERVICE_ACCOUNT_ID}` — service account ID from step 1

Then create the gateway:

```bash
yc serverless api-gateway create \
	--name bridge-gw \
	--spec spec.yaml
```

Get the gateway domain:

```bash
GW_DOMAIN=$(yc serverless api-gateway get bridge-gw --format json | jq -r .domain)
echo "Gateway: wss://$GW_DOMAIN"
```

### 4. Configure and Run the Adapter

Set `bridge.url` in `adapter.config.yaml` to `wss://<GW_DOMAIN>/_upstream`, and set `bridge.authToken` to the same secret used for `AUTH_TOKEN`.

```bash
cd adapter
go build -o adapter ./cmd/adapter
./adapter adapter.config.yaml
```

### 5. Connect Clients

Clients connect to API Gateway using any path:

```
wss://<GW_DOMAIN>/your/path
```

The path is passed to the target server unchanged. In other words, `wss://gw.example.com/chat` makes the adapter connect to `ws://127.0.0.1:9090/chat`.

View logs:

```bash
yc logging read --folder-id $(yc config get folder-id) --follow
```

---

## Yandex Cloud Limits

| Limit | Value |
|-------|-------|
| Maximum WebSocket connection lifetime | 60 minutes |
| Idle timeout (no messages) | 10 minutes |
| Maximum message size | 128 KB |
| Maximum frame size | 32 KB |
| Function execution timeout | 10 seconds (configurable) |

Set `pingIntervalMs` to less than 10 minutes to avoid idle disconnections.

## License

WTFPL, see [LICENSE](LICENSE).