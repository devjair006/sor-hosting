package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMissingMetricIsNil(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
	}))
	defer s.Close()
	c := New(s.URL, "node:9100", "/")
	v, e := c.Instant(context.Background(), "up")
	if e != nil || v != nil {
		t.Fatalf("expected missing metric, got %v %v", v, e)
	}
}
