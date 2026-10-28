package config
import("encoding/base64";
"errors";
"os";
"path/filepath";
"sor.hosting/shared/agentapi")
type Config struct{ DockerDataDir string `json:"docker_data_dir"`;
 Socket string `json:"socket"`;
PeerUID uint32 `json:"peer_uid"`;
PeerGID uint32 `json:"peer_gid"`;
StateDir string `json:"state_dir"`;
BackupDir string `json:"backup_dir"`;
NginxDir string `json:"nginx_dir"`;
ACMEWebroot string `json:"acme_webroot"`;
CertbotEmail string `json:"certbot_email"`;
PublicIPs []string `json:"public_ips"`;
ProtectedIDs []string `json:"protected_ids"`;
Adoptions map[string]agentapi.SiteResources `json:"adoptions"`;
KeyFile string `json:"key_file"`;
WordPressImage string `json:"wordpress_image"`;
MariaDBImage string `json:"mariadb_image"`;
CLIImage string `json:"cli_image"`;
Key []byte `json:"-"` }
func Load(path string)(Config,error){var c Config;
b,e:=os.ReadFile(path);
if e!=nil{return c,e};
if e=agentapi.Decode(b,&c);
e!=nil{return c,e};
for _,p:=range []string{c.Socket,c.StateDir,c.BackupDir,c.NginxDir,c.ACMEWebroot,c.KeyFile}{if !filepath.IsAbs(p){return c,errors.New("agent paths must be absolute")}};
b,e=os.ReadFile(c.KeyFile);
if e!=nil{return c,e};
c.Key,e=base64.StdEncoding.DecodeString(string(bytesTrim(b)));
if e!=nil||len(c.Key)!=32{return c,errors.New("agent key must encode 32 bytes")};
if c.PeerUID==0||c.PeerGID==0{return c,errors.New("root API peer is not allowed")};
return c,nil}
func bytesTrim(b []byte)[]byte{for len(b)>0&&(b[len(b)-1]=='\n'||b[len(b)-1]=='\r'){b=b[:len(b)-1]};
return b}
