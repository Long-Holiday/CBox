package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	cboxErr "cbox/internal/errors"
	pkgApi "cbox/pkg/api"
)

type Client struct {
	http       *http.Client
	baseURL    string
	socketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{
		http:       NewUnixSocketClient(socketPath),
		baseURL:    "http://unix",
		socketPath: socketPath,
	}
}

func NewTCPClient(endpoint string) *Client {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	return &Client{
		http:    &http.Client{Timeout: 60 * http.DefaultClient.Timeout},
		baseURL: endpoint,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, target any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cbox daemon unreachable (socket=%s): %w", c.socketPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp pkgApi.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("%w: %s", cboxErr.ErrNotFound, errResp.Error)
			}
			if resp.StatusCode == http.StatusConflict {
				return fmt.Errorf("%w: %s", cboxErr.ErrConflict, errResp.Error)
			}
			return fmt.Errorf("daemon error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("daemon request failed with status %d", resp.StatusCode)
	}

	if target != nil && resp.StatusCode != http.StatusNoContent {
		return json.NewDecoder(resp.Body).Decode(target)
	}

	return nil
}

// System
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/v1/ping", nil, nil)
}

func (c *Client) Version(ctx context.Context) (*pkgApi.VersionResponse, error) {
	var v pkgApi.VersionResponse
	err := c.do(ctx, http.MethodGet, "/v1/version", nil, &v)
	return &v, err
}

func (c *Client) GetEvents(ctx context.Context, limit int, objType, objID string) ([]pkgApi.EventResponse, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if objType != "" {
		q.Set("object_type", objType)
	}
	if objID != "" {
		q.Set("object_id", objID)
	}

	path := "/v1/events"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var events []pkgApi.EventResponse
	err := c.do(ctx, http.MethodGet, path, nil, &events)
	return events, err
}

// Containers
func (c *Client) CreateContainer(ctx context.Context, req pkgApi.ContainerCreateRequest) (*pkgApi.ContainerResponse, error) {
	var resp pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodPost, "/v1/containers", req, &resp)
	return &resp, err
}

func (c *Client) ListContainers(ctx context.Context) ([]pkgApi.ContainerResponse, error) {
	var resp []pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodGet, "/v1/containers", nil, &resp)
	return resp, err
}

func (c *Client) GetContainer(ctx context.Context, idOrName string) (*pkgApi.ContainerResponse, error) {
	var resp pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodGet, "/v1/containers/"+url.PathEscape(idOrName), nil, &resp)
	return &resp, err
}

func (c *Client) StartContainer(ctx context.Context, idOrName string, detach bool) (*pkgApi.ContainerResponse, error) {
	var resp pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodPost, "/v1/containers/"+url.PathEscape(idOrName)+"/start", pkgApi.ContainerStartRequest{Detach: detach}, &resp)
	return &resp, err
}

func (c *Client) StopContainer(ctx context.Context, idOrName string, timeoutSeconds int) (*pkgApi.ContainerResponse, error) {
	var resp pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodPost, "/v1/containers/"+url.PathEscape(idOrName)+"/stop", pkgApi.ContainerStopRequest{TimeoutSeconds: timeoutSeconds}, &resp)
	return &resp, err
}

func (c *Client) RestartContainer(ctx context.Context, idOrName string, timeoutSeconds int) (*pkgApi.ContainerResponse, error) {
	var resp pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodPost, "/v1/containers/"+url.PathEscape(idOrName)+"/restart", pkgApi.ContainerStopRequest{TimeoutSeconds: timeoutSeconds}, &resp)
	return &resp, err
}

func (c *Client) RemoveContainer(ctx context.Context, idOrName string, force bool) error {
	path := "/v1/containers/" + url.PathEscape(idOrName)
	if force {
		path += "?force=true"
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) GetLogs(ctx context.Context, idOrName string) (*pkgApi.LogsResponse, error) {
	var resp pkgApi.LogsResponse
	err := c.do(ctx, http.MethodGet, "/v1/containers/"+url.PathEscape(idOrName)+"/logs", nil, &resp)
	return &resp, err
}

func (c *Client) GetStats(ctx context.Context, idOrName string) (*pkgApi.StatsResponse, error) {
	var resp pkgApi.StatsResponse
	err := c.do(ctx, http.MethodGet, "/v1/containers/"+url.PathEscape(idOrName)+"/stats", nil, &resp)
	return &resp, err
}

func (c *Client) Exec(ctx context.Context, idOrName string, cmd []string, tty bool) (*pkgApi.ExecResponse, error) {
	var resp pkgApi.ExecResponse
	err := c.do(ctx, http.MethodPost, "/v1/containers/"+url.PathEscape(idOrName)+"/exec", pkgApi.ExecRequest{Command: cmd, Tty: tty}, &resp)
	return &resp, err
}

func (c *Client) GetTransportInfo(ctx context.Context, idOrName string) (*pkgApi.TransportInfoResponse, error) {
	var resp pkgApi.TransportInfoResponse
	err := c.do(ctx, http.MethodGet, "/v1/containers/"+url.PathEscape(idOrName)+"/transport", nil, &resp)
	return &resp, err
}

// Images
func (c *Client) BuildImage(ctx context.Context, contextDir, cboxfile, tag string) (*pkgApi.ImageResponse, error) {
	var resp pkgApi.ImageResponse
	err := c.do(ctx, http.MethodPost, "/v1/images/build", pkgApi.ImageBuildRequest{
		ContextDir: contextDir,
		Cboxfile:   cboxfile,
		Tag:        tag,
	}, &resp)
	return &resp, err
}

func (c *Client) ListImages(ctx context.Context) ([]pkgApi.ImageResponse, error) {
	var resp []pkgApi.ImageResponse
	err := c.do(ctx, http.MethodGet, "/v1/images", nil, &resp)
	return resp, err
}

func (c *Client) GetImage(ctx context.Context, idOrTag string) (*pkgApi.ImageResponse, error) {
	var resp pkgApi.ImageResponse
	err := c.do(ctx, http.MethodGet, "/v1/images/"+url.PathEscape(idOrTag), nil, &resp)
	return &resp, err
}

func (c *Client) DeleteImage(ctx context.Context, idOrTag string) error {
	return c.do(ctx, http.MethodDelete, "/v1/images/"+url.PathEscape(idOrTag), nil, nil)
}

// Volumes
func (c *Client) CreateVolume(ctx context.Context, name, source, mode string) (*pkgApi.VolumeResponse, error) {
	var resp pkgApi.VolumeResponse
	err := c.do(ctx, http.MethodPost, "/v1/volumes", pkgApi.VolumeCreateRequest{
		Name:   name,
		Source: source,
		Mode:   mode,
	}, &resp)
	return &resp, err
}

func (c *Client) ListVolumes(ctx context.Context) ([]pkgApi.VolumeResponse, error) {
	var resp []pkgApi.VolumeResponse
	err := c.do(ctx, http.MethodGet, "/v1/volumes", nil, &resp)
	return resp, err
}

func (c *Client) GetVolume(ctx context.Context, name string) (*pkgApi.VolumeResponse, error) {
	var resp pkgApi.VolumeResponse
	err := c.do(ctx, http.MethodGet, "/v1/volumes/"+url.PathEscape(name), nil, &resp)
	return &resp, err
}

func (c *Client) DeleteVolume(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/v1/volumes/"+url.PathEscape(name), nil, nil)
}

// Contexts
func (c *Client) CreateContext(ctx context.Context, req pkgApi.ContextCreateRequest) (*pkgApi.ContextResponse, error) {
	var resp pkgApi.ContextResponse
	err := c.do(ctx, http.MethodPost, "/v1/contexts", req, &resp)
	return &resp, err
}

func (c *Client) ListContexts(ctx context.Context) ([]pkgApi.ContextResponse, error) {
	var resp []pkgApi.ContextResponse
	err := c.do(ctx, http.MethodGet, "/v1/contexts", nil, &resp)
	return resp, err
}

func (c *Client) GetContext(ctx context.Context, name string) (*pkgApi.ContextResponse, error) {
	var resp pkgApi.ContextResponse
	err := c.do(ctx, http.MethodGet, "/v1/contexts/"+url.PathEscape(name), nil, &resp)
	return &resp, err
}

func (c *Client) UseContext(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodPost, "/v1/contexts/"+url.PathEscape(name)+"/use", nil, nil)
}

func (c *Client) DeleteContext(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/v1/contexts/"+url.PathEscape(name), nil, nil)
}

// Compose
func (c *Client) ComposeUp(ctx context.Context, file string, detach bool) ([]pkgApi.ContainerResponse, error) {
	var resp []pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodPost, "/v1/compose/up", map[string]any{"file": file, "detach": detach}, &resp)
	return resp, err
}

func (c *Client) ComposeDown(ctx context.Context, file string) error {
	return c.do(ctx, http.MethodPost, "/v1/compose/down", map[string]any{"file": file}, nil)
}

func (c *Client) ComposePs(ctx context.Context, file string) ([]pkgApi.ContainerResponse, error) {
	path := "/v1/compose/ps"
	if file != "" {
		path += "?file=" + url.QueryEscape(file)
	}
	var resp []pkgApi.ContainerResponse
	err := c.do(ctx, http.MethodGet, path, nil, &resp)
	return resp, err
}

func (c *Client) ComposeLogs(ctx context.Context, file string) (map[string]string, error) {
	path := "/v1/compose/logs"
	if file != "" {
		path += "?file=" + url.QueryEscape(file)
	}
	var logs map[string]string
	err := c.do(ctx, http.MethodGet, path, nil, &logs)
	return logs, err
}
