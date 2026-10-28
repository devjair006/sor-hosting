package agentapi

import (
	"encoding/json"
	"testing"
)

func TestUnknownOperationRejected(t *testing.T) {
	r := OperationRequest{ID: "0123456789abcdef", ServerID: "local", ResourceID: "0123456789abcdef", Kind: "shell", Payload: json.RawMessage(`{}`)}
	if r.Validate() == nil {
		t.Fatal("arbitrary command accepted")
	}
}
func TestUnknownFieldsRejected(t *testing.T) {
	var v ContainerInput
	if Decode([]byte(`{"action":"start","cmd":"sh"}`), &v) == nil {
		t.Fatal("unknown field accepted")
	}
}
