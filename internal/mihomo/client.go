package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"github.com/coder/websocket"
)

const DefaultTimeout = 10 * time.Second

// Client is an HTTP and WebSocket client for a mihomo controller.
type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

// NewClient creates a client for an HTTP(S) controller on the loopback
// interface. The timeout covers API requests and WebSocket handshakes.
func NewClient(baseURL, secret string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse mihomo controller URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("mihomo controller URL must use http or https")
	}
	if parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" {
		return nil, fmt.Errorf("mihomo controller URL must be an absolute HTTP URL without user information")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("mihomo controller URL must not contain a path")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, fmt.Errorf("mihomo controller URL must not contain a query or fragment")
	}
	if !isLoopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("mihomo controller URL must use a loopback address")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || !validPort(number) {
			return nil, fmt.Errorf("mihomo controller URL has an invalid port: %q", port)
		}
	}

	parsed.Path = ""
	parsed.RawPath = ""
	return &Client{
		baseURL: strings.TrimSuffix(parsed.String(), "/"),
		secret:  secret,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			// API credentials must never be forwarded through a redirect.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *Client) Version(ctx context.Context) (model.Version, error) {
	var version model.Version
	err := c.doJSON(ctx, http.MethodGet, "/version", nil, &version)
	return version, err
}

func (c *Client) Config(ctx context.Context) (model.RunningConfig, error) {
	var response struct {
		Port               int       `json:"port"`
		SocksPort          int       `json:"socks-port"`
		MixedPort          int       `json:"mixed-port"`
		Mode               string    `json:"mode"`
		LogLevel           string    `json:"log-level"`
		AllowLAN           bool      `json:"allow-lan"`
		IPv6               bool      `json:"ipv6"`
		TUN                model.TUN `json:"tun"`
		ExternalController string    `json:"external-controller"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/configs", nil, &response); err != nil {
		return model.RunningConfig{}, err
	}
	return model.RunningConfig{
		Port:               response.Port,
		SocksPort:          response.SocksPort,
		MixedPort:          response.MixedPort,
		Mode:               response.Mode,
		LogLevel:           response.LogLevel,
		AllowLAN:           response.AllowLAN,
		IPv6:               response.IPv6,
		TUN:                response.TUN,
		ExternalController: response.ExternalController,
	}, nil
}

func (c *Client) SetProxyMode(ctx context.Context, mode model.ProxyMode) error {
	return c.do(ctx, http.MethodPatch, "/configs", map[string]model.ProxyMode{"mode": mode}, nil)
}

func (c *Client) Proxies(ctx context.Context) (model.Proxies, error) {
	var proxies model.Proxies
	err := c.doJSON(ctx, http.MethodGet, "/proxies", nil, &proxies)
	return proxies, err
}

func (c *Client) SelectProxy(ctx context.Context, group, name string) error {
	path := "/proxies/" + url.PathEscape(group)
	return c.do(ctx, http.MethodPut, path, map[string]string{"name": name}, nil)
}

func (c *Client) Connections(ctx context.Context) (model.Connections, error) {
	var connections model.Connections
	err := c.doJSON(ctx, http.MethodGet, "/connections", nil, &connections)
	return connections, err
}

func (c *Client) Delay(ctx context.Context, testURL, name string, timeout int) (model.DelayResult, error) {
	query := url.Values{}
	query.Set("url", testURL)
	query.Set("timeout", strconv.Itoa(timeout))
	path := "/proxies/" + url.PathEscape(name) + "/delay?" + query.Encode()

	var result model.DelayResult
	err := c.doJSON(ctx, http.MethodGet, path, nil, &result)
	return result, err
}

// ExitIP asks an external echo service through the local mixed proxy and
// returns the observed public IPv4 or IPv6 address.
func (c *Client) ExitIP(ctx context.Context, mixedPort int) (string, error) {
	if !validPort(mixedPort) {
		return "", errors.New("mihomo mixed port is invalid")
	}
	proxyURL := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", mixedPort)}
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		return "", fmt.Errorf("create exit IP request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("check exit IP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("check exit IP returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode exit IP response: %w", err)
	}
	if payload.IP == "" {
		return "", errors.New("exit IP service returned an empty address")
	}
	return payload.IP, nil
}

// Traffic streams traffic samples until ctx is canceled, the server closes the
// socket, or the returned cancel function is called. Both channels are closed
// when streaming ends; the error channel receives at most one non-cancellation
// error.
func (c *Client) Traffic(ctx context.Context) (<-chan model.Traffic, <-chan error, context.CancelFunc, error) {
	return websocketStream[model.Traffic](ctx, c, "/traffic", func(data []byte) (model.Traffic, error) {
		var traffic model.Traffic
		err := json.Unmarshal(data, &traffic)
		return traffic, err
	})
}

// Logs streams log payloads at the requested level until canceled or closed.
// Both returned channels are closed when streaming ends.
func (c *Client) Logs(ctx context.Context, level string) (<-chan string, <-chan error, context.CancelFunc, error) {
	path := "/logs"
	if level != "" {
		path += "?level=" + url.QueryEscape(level)
	}
	return websocketStream[string](ctx, c, path, func(data []byte) (string, error) {
		var message struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}
		if err := json.Unmarshal(data, &message); err != nil {
			return "", err
		}
		return message.Payload, nil
	})
}

func websocketStream[T any](ctx context.Context, client *Client, path string, decode func([]byte) (T, error)) (<-chan T, <-chan error, context.CancelFunc, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	headers := client.authorizationHeaders()
	conn, _, err := websocket.Dial(streamCtx, client.websocketURL(path), &websocket.DialOptions{
		HTTPClient: client.httpClient,
		HTTPHeader: headers,
	})
	if err != nil {
		cancel()
		return nil, nil, cancel, fmt.Errorf("connect to mihomo websocket: %w", err)
	}

	messages := make(chan T, 16)
	errors := make(chan error, 1)
	go func() {
		defer cancel()
		defer close(messages)
		defer close(errors)
		defer conn.CloseNow()
		for {
			messageType, data, err := conn.Read(streamCtx)
			if err != nil {
				if streamCtx.Err() != nil {
					return
				}
				status := websocket.CloseStatus(err)
				if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
					return
				}
				select {
				case errors <- fmt.Errorf("read mihomo websocket: %w", err):
				default:
				}
				return
			}
			if messageType != websocket.MessageText {
				select {
				case errors <- fmt.Errorf("read mihomo websocket: unexpected message type %s", messageType):
				default:
				}
				return
			}
			message, err := decode(data)
			if err != nil {
				select {
				case errors <- fmt.Errorf("decode mihomo websocket message: %w", err):
				default:
				}
				return
			}
			select {
			case messages <- message:
			case <-streamCtx.Done():
				return
			}
		}
	}()
	return messages, errors, cancel, nil
}

func (c *Client) websocketURL(path string) string {
	endpoint, _ := url.Parse(c.baseURL)
	switch endpoint.Scheme {
	case "http":
		endpoint.Scheme = "ws"
	case "https":
		endpoint.Scheme = "wss"
	}
	endpoint.Path = path
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	if strings.Contains(path, "?") {
		endpoint.Path, endpoint.RawQuery, _ = strings.Cut(path, "?")
	}
	return endpoint.String()
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, output any) error {
	return c.do(ctx, method, path, body, output)
}

func (c *Client) do(ctx context.Context, method, path string, body, output any) error {
	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode mihomo request: %w", err)
		}
		requestBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return fmt.Errorf("create mihomo request: %w", err)
	}
	req.Header = c.authorizationHeaders()
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send mihomo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return mihomoResponseError(resp)
	}
	if output == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode mihomo response (%s): %w", resp.Status, err)
	}
	return nil
}

func (c *Client) authorizationHeaders() http.Header {
	headers := make(http.Header)
	if c.secret != "" {
		headers.Set("Authorization", "Bearer "+c.secret)
	}
	return headers
}

// APIError describes a non-successful mihomo HTTP response.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("mihomo API returned %s: %s", http.StatusText(e.StatusCode), e.Message)
	}
	return fmt.Sprintf("mihomo API returned %s", http.StatusText(e.StatusCode))
}

func mihomoResponseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	message := strings.TrimSpace(string(data))
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Message != "" {
		message = payload.Message
	}
	return &APIError{StatusCode: resp.StatusCode, Message: message}
}
