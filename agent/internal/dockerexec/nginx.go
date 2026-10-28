package dockerexec

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"sor.hosting/agent/internal/executil"
)

type NginxAPI interface {
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ExecCreate(context.Context, string, client.ExecCreateOptions) (client.ExecCreateResult, error)
	ExecAttach(context.Context, string, client.ExecAttachOptions) (client.ExecAttachResult, error)
	ExecInspect(context.Context, string, client.ExecInspectOptions) (client.ExecInspectResult, error)
}
type Bind struct {
	Source, Target string
	Writable       bool
}
type Nginx struct {
	API       NginxAPI
	Container string
	Network   string
	Mounts    []Bind
}

func (n *Nginx) Inspect(ctx context.Context) (client.ContainerInspectResult, error) {
	v, e := n.API.ContainerInspect(ctx, n.Container, client.ContainerInspectOptions{})
	if e != nil {
		return v, e
	}
	c := v.Container
	if c.ID == "" || (c.ID != n.Container && strings.TrimPrefix(c.Name, "/") != n.Container) || c.State == nil || !c.State.Running || c.State.Restarting || c.State.Paused {
		return v, errors.New("configured Nginx container is not running")
	}
	if n.Network != "" && (c.NetworkSettings == nil || c.NetworkSettings.Networks[n.Network] == nil || c.NetworkSettings.Networks[n.Network].NetworkID == "") {
		return v, errors.New("Nginx is not connected to the configured proxy network")
	}
	for _, want := range n.Mounts {
		matched := false
		for _, got := range c.Mounts {
			if string(got.Type) == "bind" && filepath.Clean(got.Source) == want.Source && got.Destination == want.Target && (!want.Writable || got.RW) {
				matched = true
			}
		}
		if !matched {
			return v, errors.New("Nginx shared bind mount does not match explicit configuration")
		}
	}
	return v, nil
}
func (n *Nginx) Alive(ctx context.Context) bool { _, e := n.Inspect(ctx); return e == nil }
func (n *Nginx) Run(ctx context.Context, name string, args []string, in io.Reader, out io.Writer) error {
	allowed := len(args) == 1 && (args[0] == "-T" || args[0] == "-t") || len(args) == 2 && args[0] == "-s" && args[1] == "reload"
	if name != "nginx" || !allowed || in != nil {
		return errors.New("Nginx exec not allowed")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	v, e := n.Inspect(ctx)
	if e != nil {
		return e
	}
	ex, e := n.API.ExecCreate(ctx, v.Container.ID, client.ExecCreateOptions{Cmd: append([]string{"nginx"}, args...), AttachStdout: true, AttachStderr: true})
	if e != nil {
		return e
	}
	r, e := n.API.ExecAttach(ctx, ex.ID, client.ExecAttachOptions{})
	if e != nil {
		return e
	}
	defer r.Close()
	stop := context.AfterFunc(ctx, r.Close)
	defer stop()
	if out == nil {
		out = &executil.Limited{Max: 4 << 20}
	}
	if _, e = stdcopy.StdCopy(out, out, r.Reader); e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("Nginx exec output failed or exceeded limit")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	for {
		x, e := n.API.ExecInspect(ctx, ex.ID, client.ExecInspectOptions{})
		if e != nil {
			return e
		}
		if !x.Running {
			if x.ExitCode != 0 {
				return errors.New("Nginx exec returned nonzero exit status")
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
