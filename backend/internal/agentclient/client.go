package agentclient
import("bytes";
"context";
"encoding/json";
"fmt";
"io";
"net";
"net/http";
"strconv";
"time";
"sor.hosting/shared/agentapi")
type Client struct{HTTP *http.Client}
func New(path string)*Client{return &Client{HTTP:&http.Client{Transport:&http.Transport{DialContext:func(ctx context.Context,_,_ string)(net.Conn,error){return (&net.Dialer{Timeout:5*time.Second}).DialContext(ctx,"unix",path)},MaxIdleConns:4,IdleConnTimeout:30*time.Second}}}}
func(c *Client)request(ctx context.Context,method,path string,in any)(*http.Response,error){var b io.Reader;
if in!=nil{data,e:=json.Marshal(in);
if e!=nil{return nil,e};
b=bytes.NewReader(data)};
r,e:=http.NewRequestWithContext(ctx,method,"http://agent"+path,b);
if e!=nil{return nil,e};
r.Header.Set("Content-Type","application/json");
res,e:=c.HTTP.Do(r);
if e!=nil{return nil,fmt.Errorf("agent unavailable: %w",e)};
if res.StatusCode>=300{res.Body.Close();
return nil,fmt.Errorf("agent rejected request (%d)",res.StatusCode)};
return res,nil}
func(c *Client)do(ctx context.Context,method,path string,in,out any)error{ctx,cancel:=context.WithTimeout(ctx,15*time.Second);
defer cancel();
r,e:=c.request(ctx,method,path,in);
if e!=nil{return e};
defer r.Body.Close();
return json.NewDecoder(io.LimitReader(r.Body,4<<20)).Decode(out)}
func(c *Client)Status(ctx context.Context)(v agentapi.Status,e error){e=c.do(ctx,"GET","/status",nil,&v);
return}
func(c *Client)Containers(ctx context.Context)(v []agentapi.Container,e error){e=c.do(ctx,"GET","/containers",nil,&v);
return}
func(c *Client)Logs(ctx context.Context,in agentapi.LogsRequest)(v agentapi.LogsResult,e error){if !agentapi.OpaqueID.MatchString(in.ContainerID){return v,fmt.Errorf("invalid ID")};
e=c.do(ctx,"GET","/containers/"+in.ContainerID+"/logs?tail="+strconv.Itoa(in.Tail),nil,&v);
return}
func(c *Client)Submit(ctx context.Context,in agentapi.OperationRequest)(v agentapi.Operation,e error){if e=in.Validate();
e!=nil{return};
e=c.do(ctx,"POST","/operations",in,&v);
return}
func(c *Client)Operation(ctx context.Context,id string)(v agentapi.Operation,e error){if !agentapi.OpaqueID.MatchString(id){return v,fmt.Errorf("invalid ID")};
e=c.do(ctx,"GET","/operations/"+id,nil,&v);
return}
func(c *Client)Download(ctx context.Context,id string)(io.ReadCloser,agentapi.BackupFile,error){if !agentapi.OpaqueID.MatchString(id){return nil,agentapi.BackupFile{},fmt.Errorf("invalid ID")};
r,e:=c.request(ctx,"GET","/backups/"+id+"/download",nil);
if e!=nil{return nil,agentapi.BackupFile{},e};
return r.Body,agentapi.BackupFile{Name:id+".tar.gz",Size:r.ContentLength},nil}
