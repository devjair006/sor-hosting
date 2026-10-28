package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Base, Instance, Filesystem string
	HTTP                       *http.Client
}

func New(base, instance, fs string) *Client {
	return &Client{strings.TrimRight(base, "/"), instance, fs, &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) query(ctx context.Context, path string, params url.Values) ([]json.RawMessage, error) {
	if c.Base == "" {
		return nil, errors.New("Prometheus sin configurar")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", c.Base+"/api/v1/"+path+"?"+params.Encode(), nil)
	if e != nil {
		return nil, e
	}
	res, e := c.HTTP.Do(req)
	if e != nil {
		return nil, errors.New("Prometheus no disponible")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Prometheus rechazó la consulta")
	}
	var v struct {
		Status string `json:"status"`
		Data   struct {
			Result []json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&v); e != nil || v.Status != "success" {
		return nil, errors.New("respuesta de Prometheus inválida")
	}
	return v.Data.Result, nil
}
func number(p []json.RawMessage) *float64 {
	if len(p) != 2 {
		return nil
	}
	var text string
	if json.Unmarshal(p[1], &text) != nil {
		return nil
	}
	v, e := strconv.ParseFloat(text, 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func (c *Client) Instant(ctx context.Context, q string) (*float64, error) {
	rows, e := c.query(ctx, "query", url.Values{"query": {q}})
	if e != nil {
		return nil, e
	}
	if len(rows) != 1 {
		return nil, nil
	}
	var row struct {
		Value []json.RawMessage `json:"value"`
	}
	if e = json.Unmarshal(rows[0], &row); e != nil {
		return nil, e
	}
	return number(row.Value), nil
}
func (c *Client) expressions() map[string]string {
	s := fmt.Sprintf(`instance=%q`, c.Instance)
	fs := fmt.Sprintf(`%s,mountpoint=%q,fstype!~"tmpfs|overlay|squashfs|devtmpfs"`, s, c.Filesystem)
	return map[string]string{"up": "min(up{" + s + "})", "cpu": "100*(1-avg(rate(node_cpu_seconds_total{" + s + `,mode="idle"}[5m])))`, "memory_pct": "100*(1-node_memory_MemAvailable_bytes{" + s + "}/node_memory_MemTotal_bytes{" + s + "})", "memory_total": "node_memory_MemTotal_bytes{" + s + "}", "memory_used": "node_memory_MemTotal_bytes{" + s + "}-node_memory_MemAvailable_bytes{" + s + "}", "disk_total": "node_filesystem_size_bytes{" + fs + "}", "disk_used": "node_filesystem_size_bytes{" + fs + "}-node_filesystem_avail_bytes{" + fs + "}", "traffic": "sum(rate(node_network_receive_bytes_total{" + s + `,device!~"lo|veth.*|docker.*|br-.*"}[5m]))+sum(rate(node_network_transmit_bytes_total{` + s + `,device!~"lo|veth.*|docker.*|br-.*"}[5m]))`}
}
func (c *Client) Summary(ctx context.Context) (map[string]*float64, bool) {
	out := map[string]*float64{"cpu": nil, "memory_pct": nil, "memory_used": nil, "memory_total": nil, "disk_used": nil, "disk_total": nil, "traffic": nil}
	if c.Base == "" {
		return out, false
	}
	expr := c.expressions()
	up, e := c.Instant(ctx, expr["up"])
	if e != nil {
		return out, false
	}
	if up == nil || *up == 0 {
		return out, true
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for name := range out {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			v, _ := c.Instant(ctx, expr[name])
			mu.Lock()
			out[name] = v
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	return out, true
}

type Point struct {
	Timestamp float64  `json:"timestamp"`
	Value     *float64 `json:"value"`
}
type Series struct {
	Name   string  `json:"name"`
	Unit   string  `json:"unit"`
	Points []Point `json:"points"`
}
type History struct {
	Series     []Series `json:"series"`
	ObservedAt string   `json:"observed_at"`
	Stale      bool     `json:"stale"`
}

func (c *Client) History(ctx context.Context, period string) (History, error) {
	out := History{Series: []Series{}, Stale: true}
	spans := map[string]time.Duration{"1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour}
	span, ok := spans[period]
	if !ok {
		return out, errors.New("intervalo inválido")
	}
	end := time.Now()
	step := int(span.Seconds()) / 550
	if step < 30 {
		step = 30
	}
	last := float64(0)
	expr := c.expressions()
	for _, name := range []string{"cpu", "memory"} {
		q := expr[name]
		if name == "memory" {
			q = expr["memory_pct"]
		}
		rows, e := c.query(ctx, "query_range", url.Values{"query": {q}, "start": {strconv.FormatInt(end.Add(-span).Unix(), 10)}, "end": {strconv.FormatInt(end.Unix(), 10)}, "step": {strconv.Itoa(step)}})
		if e != nil {
			return out, e
		}
		s := Series{Name: name, Unit: "%", Points: []Point{}}
		if len(rows) == 1 {
			var row struct {
				Values [][]json.RawMessage `json:"values"`
			}
			if json.Unmarshal(rows[0], &row) != nil {
				return out, errors.New("invalid series")
			}
			if len(row.Values) > 600 {
				return out, errors.New("too many points")
			}
			for _, p := range row.Values {
				if len(p) != 2 {
					continue
				}
				var ts float64
				if json.Unmarshal(p[0], &ts) != nil {
					continue
				}
				s.Points = append(s.Points, Point{Timestamp: ts, Value: number(p)})
				if ts > last {
					last = ts
				}
			}
		}
		out.Series = append(out.Series, s)
	}
	if last > 0 {
		out.ObservedAt = time.Unix(int64(last), 0).UTC().Format(time.RFC3339)
		out.Stale = time.Since(time.Unix(int64(last), 0)) > 2*time.Minute
	}
	return out, nil
}
