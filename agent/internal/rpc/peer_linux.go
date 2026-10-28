package rpc
import("context";
"net";
"golang.org/x/sys/unix")
type peerKey struct{}
type peer struct{UID,GID uint32;
Valid bool}
func credentials(ctx context.Context,c net.Conn)context.Context{p:=peer{};
if u,ok:=c.(*net.UnixConn);
ok{raw,e:=u.SyscallConn();
if e==nil{e=raw.Control(func(fd uintptr){cred,err:=unix.GetsockoptUcred(int(fd),unix.SOL_SOCKET,unix.SO_PEERCRED);
if err==nil{p=peer{UID:cred.Uid,GID:cred.Gid,Valid:true}}});
if e!=nil{p.Valid=false}}};
return context.WithValue(ctx,peerKey{},p)}
func allowed(ctx context.Context,uid,gid uint32)bool{p,ok:=ctx.Value(peerKey{}).(peer);
return ok&&p.Valid&&p.UID==uid&&p.GID==gid}
