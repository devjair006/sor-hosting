package rpc
import("context";
"testing")
func TestUnauthorizedPeerRejected(t *testing.T){for _,p:=range []peer{{UID:0,GID:10001,Valid:true},{UID:10001,GID:0,Valid:true},{UID:10001,GID:10001,Valid:false}}{if allowed(context.WithValue(context.Background(),peerKey{},p),10001,10001){t.Fatal("unauthorized peer accepted")}};
if !allowed(context.WithValue(context.Background(),peerKey{},peer{10001,10001,true}),10001,10001){t.Fatal("authorized peer rejected")}}
