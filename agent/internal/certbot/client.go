package certbot
import("context";
"crypto/sha256";
"crypto/x509";
"encoding/hex";
"encoding/pem";
"errors";
"net";
"os";
"path/filepath";
"time";
"sor.hosting/agent/internal/executil";
"sor.hosting/shared/agentapi";
"sor.hosting/shared/domainname";
"sor.hosting/shared/netguard")
type Client struct{Webroot,Email string;
PublicIPs []string;
Runner executil.Runner}
func(c *Client)DNS(ctx context.Context,host string)bool{ips,e:=net.DefaultResolver.LookupNetIP(ctx,"ip",host);
if e!=nil||len(ips)==0{return false};
for _,ip:=range ips{ok:=false;
for _,allowed:=range c.PublicIPs{if ip.Unmap().String()==allowed&&netguard.Public(allowed){ok=true}};
if !ok{return false}};
return true}
func(c *Client)Issue(ctx context.Context,id,host string,renew bool)(*agentapi.Certificate,error){host,e:=domainname.Normalize(host);
if e!=nil{return nil,e};
if !c.DNS(ctx,host){return nil,errors.New("DNS_PENDING")};
name:="sor-"+host;
args:=[]string{"certonly","--webroot","--webroot-path",c.Webroot,"--domain",host,"--cert-name",name,"--email",c.Email,"--agree-tos","--non-interactive","--keep-until-expiring"};
if renew{args=[]string{"renew","--cert-name",name,"--non-interactive"}};
if e=c.Runner.Run(ctx,"certbot",args,nil,nil);
e!=nil{return nil,e};
b,e:=os.ReadFile(filepath.Join("/etc/letsencrypt/live",name,"fullchain.pem"));
if e!=nil{return nil,e};
block,_:=pem.Decode(b);
if block==nil{return nil,errors.New("invalid certificate")};
cert,e:=x509.ParseCertificate(block.Bytes);
if e!=nil{return nil,e};
if e=cert.VerifyHostname(host);
e!=nil{return nil,e};
if cert.NotAfter.Before(time.Now()){return nil,errors.New("expired certificate")};
sum:=sha256.Sum256(cert.Raw);
return &agentapi.Certificate{ID:id,DomainID:id,CertName:name,Hostname:host,ExpiresAt:cert.NotAfter.UTC().Format(time.RFC3339),Fingerprint:hex.EncodeToString(sum[:])},nil}
