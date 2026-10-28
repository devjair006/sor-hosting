package dockerexec

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

type Certbot struct {
	API                                *client.Client
	Image, ConfigSource, WebrootSource string
}

func (c *Certbot) Available(ctx context.Context) bool {
	_, e := c.API.ImageInspect(ctx, c.Image)
	return e == nil
}
func (c *Certbot) Run(ctx context.Context, name string, args []string, in io.Reader, out io.Writer) (err error) {
	if name != "certbot" || in != nil || len(args) == 0 || (args[0] != "certonly" && args[0] != "renew" && args[0] != "--version") {
		return errors.New("Certbot command not allowed")
	}
	for _, v := range args {
		if strings.Contains(v, "hook") && v != "--no-directory-hooks" {
			return errors.New("Certbot hooks not allowed")
		}
	}
	if !c.Available(ctx) {
		return errors.New("pinned Certbot image unavailable; pull it manually")
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	pids := int64(64)
	v, err := c.API.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name:   "sor-certbot-" + hex.EncodeToString(nonce),
		Config: &container.Config{Image: c.Image, Entrypoint: []string{"certbot"}, Cmd: args, Labels: map[string]string{"sor.hosting/infra": "true", "sor.hosting/certbot": "true"}},
		HostConfig: &container.HostConfig{ReadonlyRootfs: true, CapDrop: []string{"ALL"}, CapAdd: []string{"DAC_OVERRIDE"}, SecurityOpt: []string{"no-new-privileges:true"}, Resources: container.Resources{Memory: 512 << 20, NanoCPUs: 1_000_000_000, PidsLimit: &pids},
			Mounts: []mount.Mount{{Type: mount.TypeBind, Source: c.ConfigSource, Target: "/etc/letsencrypt"}, {Type: mount.TypeBind, Source: c.WebrootSource, Target: "/var/www/certbot"}},
			Tmpfs:  map[string]string{"/tmp": "rw,noexec,nosuid,size=32m", "/var/lib/letsencrypt": "rw,noexec,nosuid,size=32m", "/var/log/letsencrypt": "rw,noexec,nosuid,size=32m"},
		},
	})
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, e := c.API.ContainerRemove(cleanup, v.ID, client.ContainerRemoveOptions{Force: true})
		if e != nil && err == nil {
			err = errors.New("temporary Certbot cleanup failed")
		}
	}()
	if _, err = c.API.ContainerStart(ctx, v.ID, client.ContainerStartOptions{}); err != nil {
		return err
	}
	wait := c.API.ContainerWait(ctx, v.ID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e := <-wait.Error:
		return e
	case result := <-wait.Result:
		if result.StatusCode != 0 || result.Error != nil {
			return errors.New("Certbot container failed; inspect ACME configuration and DNS")
		}
		return nil
	}
}
