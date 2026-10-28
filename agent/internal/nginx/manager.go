package nginx
import("context";
"encoding/json";
"time";
"errors";
"fmt";
"os";
"path/filepath";
"strconv";
"strings";
"regexp";
"sync";
"sor.hosting/agent/internal/executil";
"sor.hosting/shared/domainname")
type Manager struct{Dir,Webroot string;
Runner executil.Executor;
mu sync.Mutex}
func(m *Manager)Apply(ctx context.Context,host string,port uint16,certName string)error{m.mu.Lock();
defer m.mu.Unlock();
host,e:=domainname.Normalize(host);
if e!=nil||port==0{return errors.New("invalid virtual host")};
if certName!=""{if certName!="sor-"+host{return errors.New("invalid certificate reference")}};
if e=PrivatePath(m.Dir);
e!=nil{return e};
if e=PrivatePath(m.Webroot);
e!=nil{return e};
if e=m.checkInclude(ctx,host);
e!=nil{return e};
path:=filepath.Join(m.Dir,host+".conf");
if st,err:=os.Lstat(path);
err==nil&&st.Mode()&os.ModeSymlink!=0{return errors.New("symlink config rejected")};
old,oldErr:=os.ReadFile(path);
if oldErr!=nil&&!os.IsNotExist(oldErr){return oldErr};
upstream:="http://127.0.0.1:"+strconv.Itoa(int(port));
location:=fmt.Sprintf("location / { proxy_pass %s; proxy_set_header Host $host; proxy_set_header X-Forwarded-Proto $scheme; proxy_set_header X-Forwarded-For $remote_addr; proxy_connect_timeout 5s; proxy_read_timeout 60s; }",upstream)
 conf:=fmt.Sprintf("# Managed by SOR Hosting\nserver { listen 80; server_name %s; location ^~ /.well-known/acme-challenge/ { root %q; } %s }\n",host,m.Webroot,location);
if certName!=""{conf=fmt.Sprintf("# Managed by SOR Hosting\nserver { listen 80; server_name %s; location ^~ /.well-known/acme-challenge/ { root %q; } location / { return 301 https://$host$request_uri; } }\nserver { listen 443 ssl; server_name %s; ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem; ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem; %s }\n",host,m.Webroot,host,certName,certName,location)}
 f,e:=os.CreateTemp(m.Dir,".sor-*.tmp");
if e!=nil{return e};
temp:=f.Name();
defer os.Remove(temp);
if e=f.Chmod(0644);
e==nil{_,e=f.WriteString(conf)};
if e==nil{e=f.Sync()};
closeErr:=f.Close();
if e==nil{e=closeErr};
if e!=nil{return e};
marker:=filepath.Join(m.Dir,".sor-"+host+".rollback");
journal,_:=json.Marshal(transaction{Host:host,Previous:old,Exists:oldErr==nil});
jf,x:=os.OpenFile(marker,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);
if x!=nil{return x};
_,x=jf.Write(journal);
if x==nil{x=jf.Sync()};
closeErr=jf.Close();
if x==nil{x=closeErr};
if x!=nil{return x};
if e=os.Rename(temp,path);
e!=nil{return e};
rollback:=func()error{if oldErr==nil{return os.WriteFile(path,old,0644)};
e:=os.Remove(path);
if os.IsNotExist(e){return nil};
return e};
if e=m.run(ctx,[]string{"-t"});
e!=nil{if x:=rollback();
x!=nil{return errors.New("RECOVERY_REQUIRED")};
_ = os.Remove(marker);
return e};
if e=m.run(ctx,[]string{"-s","reload"});
e!=nil{if x:=rollback();
x!=nil{return errors.New("RECOVERY_REQUIRED")};
recoverCtx,cancel:=context.WithTimeout(context.Background(),10*time.Second);
defer cancel();
if x:=m.run(recoverCtx,[]string{"-t"});
x==nil{if x=m.run(recoverCtx,[]string{"-s","reload"});
x==nil{_ = os.Remove(marker)}};
return e};
return os.Remove(marker)}
func(m *Manager)run(ctx context.Context,args []string)error{if m.Runner!=nil{return m.Runner.Run(ctx,"nginx",args,nil,nil)};
return (executil.Runner{}).Run(ctx,"nginx",args,nil,nil)}
func PrivatePath(path string)error{if !filepath.IsAbs(path){return errors.New("absolute path required")};
clean:=filepath.Clean(path);
resolved,e:=filepath.EvalSymlinks(clean);
if e!=nil{return e};
if resolved!=clean{return errors.New("symlink directory rejected")};
st,e:=os.Stat(clean);
if e!=nil{return e};
if !st.IsDir()||st.Mode().Perm()&0022!=0{return errors.New("directory must not be writable by group or world")};
return nil}

func(m *Manager)checkInclude(ctx context.Context,host string)error{out:=&executil.Limited{Max:4<<20};
var e error;
if m.Runner!=nil{e=m.Runner.Run(ctx,"nginx",[]string{"-T"},nil,out)}else{e=(executil.Runner{}).Run(ctx,"nginx",[]string{"-T"},nil,out)};
if e!=nil{return e};
dump:=out.Buffer.String();
included:=false;
want:="include "+m.Dir+"/*.conf;";
for _,line:=range strings.Split(dump,"\n"){line=strings.SplitN(line,"#",2)[0];
if strings.Join(strings.Fields(line)," ")==want{included=true}};
if !included{return errors.New("managed Nginx include is not enabled")};
marker:=regexp.MustCompile(`(?m)^# configuration file ([^\n]+):\n`);
matches:=marker.FindAllStringSubmatchIndex(dump,-1);
names:=regexp.MustCompile(`(?s)\bserver_name\s+([^;]+);`);
for i,match:=range matches{filename:=dump[match[2]:match[3]];
if filepath.Dir(filename)==m.Dir{continue};
end:=len(dump);
if i+1<len(matches){end=matches[i+1][0]};
section:=dump[match[1]:end];
lines:=strings.Split(section,"\n");
for j:=range lines{lines[j]=strings.SplitN(lines[j],"#",2)[0]};
for _,statement:=range names.FindAllStringSubmatch(strings.Join(lines,"\n"),-1){for _,name:=range strings.Fields(statement[1]){name=strings.Trim(name,"\"'");
matched,matchErr:=unmanagedMatch(name,host);if matchErr!=nil{return matchErr};if matched{return errors.New("domain already served by an unmanaged vhost")}}}};
return nil}

func unmanagedMatch(name,host string)(bool,error){if name==host||strings.HasPrefix(name,"*.")&&strings.HasSuffix(host,name[1:])||strings.HasSuffix(name,".*")&&strings.HasPrefix(host,name[:len(name)-1])||strings.HasPrefix(name,".")&&(host==name[1:]||strings.HasSuffix(host,name)){return true,nil};if strings.HasPrefix(name,"~"){pattern:=strings.TrimPrefix(name,"~");if strings.HasPrefix(pattern,"*"){pattern="(?i)"+strings.TrimPrefix(pattern,"*")};re,e:=regexp.Compile(pattern);if e!=nil{return false,errors.New("unmanaged regex vhost needs manual review")};return re.MatchString(host),nil};return false,nil}
