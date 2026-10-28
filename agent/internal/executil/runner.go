package executil
import("bytes";
"context";
"errors";
"fmt";
"io";
"os/exec";
"path/filepath";
"strings")
var ErrLimit=errors.New("output limit exceeded")
type Limited struct{Buffer bytes.Buffer;
Max int}
func(w *Limited)Write(p []byte)(int,error){if len(p)>w.Max-w.Buffer.Len(){n:=w.Max-w.Buffer.Len();
if n>0{w.Buffer.Write(p[:n])};
return n,ErrLimit};
return w.Buffer.Write(p)}
type Runner struct{}
func(Runner)Run(ctx context.Context,name string,args []string,in io.Reader,out io.Writer)error{if name!="docker"&&name!="nginx"&&name!="certbot"{return errors.New("executable not allowed")};
path,e:=exec.LookPath(name);
if e!=nil{return fmt.Errorf("required executable %s unavailable",name)};
path,e=filepath.Abs(path);
if e!=nil{return e};
allowed:=strings.HasPrefix(path,"/usr/bin/")||strings.HasPrefix(path,"/usr/sbin/")||name=="certbot"&&(path=="/snap/bin/certbot"||path=="/opt/certbot/bin/certbot");
if !allowed{return errors.New("untrusted executable location")};
cmd:=exec.CommandContext(ctx,path,args...);
cmd.Stdin=in;
if out==nil{out=&Limited{Max:256<<10}};
cmd.Stdout=out;
cmd.Stderr=&Limited{Max:16<<10};
if e=cmd.Run();
e!=nil{return fmt.Errorf("%s failed; inspect host service diagnostics",name)};
return nil}
