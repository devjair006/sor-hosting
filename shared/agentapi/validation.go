package agentapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

var OpaqueID = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,80}$`)

func Decode(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return errors.New("un único objeto JSON requerido")
	}
	return nil
}
func (r OperationRequest) Validate() error {
	if !OpaqueID.MatchString(r.ID) || r.ServerID != "local" || !OpaqueID.MatchString(r.ResourceID) {
		return errors.New("identificador inválido")
	}
	var v any
	switch r.Kind {
	case "container.start", "container.stop", "container.restart":
		v = &ContainerInput{}
	case "site.install":
		v = &SiteInput{}
	case "site.adopt":
		v = &AdoptInput{}
	case "domain.configure", "certificate.issue", "certificate.renew":
		v = &DomainInput{}
	case "backup.create", "backup.restore":
		v = &BackupInput{}
	default:
		return errors.New("operación no autorizada")
	}
	if e := Decode(r.Payload, v); e != nil {
		return e
	}
	if strings.HasPrefix(r.Kind, "container.") {
		if x := v.(*ContainerInput); r.Kind != "container."+x.Action {
			return errors.New("acción incompatible")
		}
	}
	return nil
}
