package config

import (
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Mode, Address, Database, Origin, AgentSocket, StaticDir, PrometheusURL, PrometheusInstance, Filesystem string
	Key                                                                                                    []byte
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func Load() (Config, error) {
	c := Config{Mode: env("SOR_MODE", "production"), Address: env("SOR_ADDRESS", "127.0.0.1:8080"), Database: env("SOR_DATABASE", "./data/panel.db"), Origin: os.Getenv("SOR_ORIGIN"), AgentSocket: env("SOR_AGENT_SOCKET", "/run/sor-hosting/agent.sock"), StaticDir: env("SOR_STATIC_DIR", "../frontend/dist"), PrometheusURL: os.Getenv("SOR_PROMETHEUS_URL"), PrometheusInstance: os.Getenv("SOR_PROMETHEUS_INSTANCE"), Filesystem: env("SOR_FILESYSTEM", "/")}
	var e error
	c.Key, e = base64.StdEncoding.DecodeString(os.Getenv("SOR_SECRET_KEY"))
	if e != nil || len(c.Key) != 32 {
		return c, errors.New("SOR_SECRET_KEY debe ser base64 de 32 bytes")
	}
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return c, errors.New("SOR_ORIGIN debe contener solo esquema y host")
	}
	if c.Mode != "development" && c.Mode != "production" {
		return c, errors.New("SOR_MODE inválido")
	}
	if c.Mode == "production" && u.Scheme != "https" {
		return c, errors.New("producción exige HTTPS")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return c, errors.New("esquema inválido")
	}
	if c.PrometheusURL != "" {
		u, e = url.Parse(c.PrometheusURL)
		if e != nil || u.Host == "" || !strings.HasPrefix(u.Scheme, "http") || u.User != nil {
			return c, errors.New("Prometheus URL inválida")
		}
		if c.PrometheusInstance == "" {
			return c, errors.New("falta SOR_PROMETHEUS_INSTANCE")
		}
	}
	return c, nil
}
