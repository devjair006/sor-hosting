package operations
import("errors";"strings";"testing")
func TestPublicFailureNeverIncludesRawSecret(t *testing.T){code,message:=publicFailure(errors.New("unexpected password=private-value"));if code!="OPERATION_FAILED"||strings.Contains(message,"private-value"){t.Fatal("raw error leaked")}}
