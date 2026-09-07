package downloader

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultFNOSGateway = "ws://127.0.0.1:5666/websocket?type=main"

type fnosGateway struct {
	conn       net.Conn
	stopCancel func() bool
	reader     *bufio.Reader
	backID     string
	index      uint16
	signKey    []byte
	uid        int64
}

type fnosGatewayCycleKey struct{}

// fnosGatewayCycle keeps a single authenticated WebSocket alive for one
// downloader poll. Stats, original-limit discovery and limit updates otherwise
// caused three independent logins in the same five-second poll.
type fnosGatewayCycle struct {
	downloaderID int64
	gateway      *fnosGateway
	err          error
}

func withFNOSGatewayCycle(ctx context.Context, d Downloader) (context.Context, func()) {
	if d.Kind != DownloaderFNOS {
		return ctx, func() {}
	}
	cycle := &fnosGatewayCycle{downloaderID: d.ID}
	return context.WithValue(ctx, fnosGatewayCycleKey{}, cycle), func() {
		if cycle.gateway != nil {
			_ = cycle.gateway.Close()
		}
	}
}

func acquireFNOSGateway(ctx context.Context, d Downloader) (*fnosGateway, func(), error) {
	if cycle, ok := ctx.Value(fnosGatewayCycleKey{}).(*fnosGatewayCycle); ok && cycle.downloaderID == d.ID {
		if cycle.gateway == nil && cycle.err == nil {
			cycle.gateway, cycle.err = dialFNOSGateway(ctx, d)
		}
		return cycle.gateway, func() {}, cycle.err
	}
	gateway, err := dialFNOSGateway(ctx, d)
	if err != nil {
		return nil, func() {}, err
	}
	return gateway, func() { _ = gateway.Close() }, nil
}

func dialFNOSGateway(ctx context.Context, d Downloader) (*fnosGateway, error) {
	address := strings.TrimSpace(d.BaseURL)
	if address == "" {
		address = defaultFNOSGateway
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "ws" || u.Host == "" {
		return nil, errors.New("飞牛网关地址必须是 ws:// 地址")
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	dialer := net.Dialer{Timeout: 8 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("连接飞牛 WebSocket 网关: %w", err)
	}
	// TCP reads/writes do not inherit DialContext cancellation. Interrupt the
	// handshake, authentication and later requests when the command is stopped.
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	connected := false
	defer func() {
		if !connected {
			stopCancel()
			_ = conn.Close()
		}
	}()
	deadline := time.Now().Add(8 * time.Second)
	if requested, ok := ctx.Deadline(); ok && requested.Before(deadline) {
		deadline = requested
	}
	_ = conn.SetDeadline(deadline)
	keyBytes := make([]byte, 16)
	if _, err = rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	websocketKey := base64.StdEncoding.EncodeToString(keyBytes)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	req.Host = u.Host
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", websocketKey)
	if err = req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取飞牛 WebSocket 握手: %w", err)
	}
	resp.Body.Close()
	expected := sha1.Sum([]byte(websocketKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(expected[:]) {
		conn.Close()
		return nil, fmt.Errorf("飞牛 WebSocket 握手失败: HTTP %d", resp.StatusCode)
	}
	gateway := &fnosGateway{conn: conn, stopCancel: stopCancel, reader: reader, backID: "0000000000000000", index: 1, uid: d.UserID}
	if err = gateway.login(ctx, d); err != nil {
		gateway.Close()
		return nil, err
	}
	connected = true
	return gateway, nil
}

func (g *fnosGateway) Close() error {
	if g == nil || g.conn == nil {
		return nil
	}
	if g.stopCancel != nil {
		g.stopCancel()
	}
	// Closing must never wait for a peer or attempt another blocking frame.
	return g.conn.Close()
}

func (g *fnosGateway) requestID() string {
	backID := g.backID
	if len(backID) != 16 {
		backID = "0000000000000000"
	}
	requestID := fmt.Sprintf("%08x%s%04x", uint32(time.Now().Unix()), backID, g.index)
	g.index++
	return requestID
}

func fnosNoSignRequest(endpoint string) bool {
	switch endpoint {
	case "encrypted", "util.getSI", "util.crypto.getRSAPub":
		return true
	default:
		return false
	}
}

func (g *fnosGateway) request(ctx context.Context, endpoint string, params map[string]any) (map[string]any, error) {
	payload := map[string]any{"req": endpoint, "reqid": g.requestID()}
	for key, value := range params {
		payload[key] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	message := data
	if len(g.signKey) > 0 && !fnosNoSignRequest(endpoint) {
		mac := hmac.New(sha256.New, g.signKey)
		_, _ = mac.Write(data)
		signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		message = append([]byte(signature), data...)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = g.conn.SetDeadline(deadline)
	} else {
		_ = g.conn.SetDeadline(time.Now().Add(15 * time.Second))
	}
	if err = g.writeFrame(1, message); err != nil {
		return nil, err
	}
	return g.readResponse(ctx, payload["reqid"].(string), endpoint)
}

func (g *fnosGateway) readResponse(ctx context.Context, requestID, endpoint string) (map[string]any, error) {
	for i := 0; i < 16; i++ {
		opcode, payload, err := g.readFrame()
		if err != nil {
			return nil, err
		}
		if opcode == 9 {
			_ = g.writeFrame(10, payload)
			continue
		}
		if opcode == 8 {
			return nil, errors.New("飞牛 WebSocket 已关闭")
		}
		if opcode != 1 {
			continue
		}
		var response map[string]any
		if err = json.Unmarshal(payload, &response); err != nil {
			continue
		}
		if id, _ := response["reqid"].(string); id != "" && id != requestID {
			continue
		}
		if result, _ := response["result"].(string); result == "fail" {
			code := int64Value(response["errno"])
			message := strings.TrimSpace(fmt.Sprint(response["errmsg"]))
			if message == "" || message == "<nil>" {
				switch code {
				case 131072:
					message = "登录请求被系统拒绝，但系统未提供具体原因"
				default:
					message = "系统未返回错误说明"
				}
			}
			debugf(ctx, "飞牛接口返回失败：端点=%s，errno=%d，字段=%v", endpoint, code, sortedFNOSKeys(response))
			return nil, fmt.Errorf("飞牛接口 %s 失败（错误码 %d）：%s", endpoint, code, message)
		}
		return response, nil
	}
	return nil, errors.New("没有收到匹配的飞牛接口响应")
}
