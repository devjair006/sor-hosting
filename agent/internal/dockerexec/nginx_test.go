package dockerexec

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type execFixture struct {
	cmd            []string
	target         string
	exit           int
	stopped, block bool
}

func (f *execFixture) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{Container: container.InspectResponse{ID: strings.Repeat("a", 64), Name: "/edge", State: &container.State{Running: !f.stopped}}}, nil
}
func (f *execFixture) ExecCreate(_ context.Context, id string, o client.ExecCreateOptions) (client.ExecCreateResult, error) {
	f.cmd = o.Cmd
	f.target = id
	return client.ExecCreateResult{ID: "exec"}, nil
}
func (f *execFixture) ExecAttach(ctx context.Context, _ string, _ client.ExecAttachOptions) (client.ExecAttachResult, error) {
	a, b := net.Pipe()
	go func() {
		defer b.Close()
		if f.block {
			<-ctx.Done()
			return
		}
		v := []byte("nginx output\n")
		h := make([]byte, 8)
		h[0] = 1
		binary.BigEndian.PutUint32(h[4:], uint32(len(v)))
		b.Write(h)
		b.Write(v)
	}()
	return client.ExecAttachResult{HijackedResponse: client.NewHijackedResponse(a, "")}, nil
}
func (f *execFixture) ExecInspect(context.Context, string, client.ExecInspectOptions) (client.ExecInspectResult, error) {
	return client.ExecInspectResult{ExitCode: f.exit}, nil
}
func TestExecUsesExactIDAndExitCode(t *testing.T) {
	f := &execFixture{}
	e := &Nginx{API: f, Container: "edge"}
	var out strings.Builder
	if err := e.Run(context.Background(), "nginx", []string{"-T"}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if f.target != strings.Repeat("a", 64) || strings.Join(f.cmd, " ") != "nginx -T" || out.String() != "nginx output\n" {
		t.Fatal(f.target, f.cmd, out.String())
	}
	f.exit = 1
	if e.Run(context.Background(), "nginx", []string{"-t"}, nil, io.Discard) == nil {
		t.Fatal("nonzero exit accepted")
	}
	if e.Run(context.Background(), "sh", []string{"-c", "anything"}, nil, nil) == nil {
		t.Fatal("arbitrary exec accepted")
	}
}
func TestDockerNginxAvailabilityAndCancellation(t *testing.T) {
	f := &execFixture{stopped: true}
	e := &Nginx{API: f, Container: "edge"}
	if e.Alive(context.Background()) {
		t.Fatal("stopped container healthy")
	}
	f.stopped = false
	f.block = true
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := e.Run(ctx, "nginx", []string{"-t"}, nil, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("cancellation did not preserve context error", err)
	}
}
