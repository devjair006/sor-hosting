package httpapi
import(
"context"
"net/http"
"time"
"sor.hosting/backend/internal/database/sqlc"
"sor.hosting/backend/internal/httpx"
)
func(a *App)dashboard(w http.ResponseWriter,r *http.Request){s,e:=a.Q.Server(r.Context());
if e!=nil{fail(w,r,e);
return};
sites,e:=a.Q.ListSites(r.Context());
if e!=nil{fail(w,r,e);
return};
audit,e:=a.Q.ListAudit(r.Context(),sqlc.ListAuditParams{Limit:4,Offset:0});
if e!=nil{fail(w,r,e);
return};
ctx,cancel:=context.WithTimeout(r.Context(),7*time.Second);
defer cancel();
status,agentErr:=a.Agent.Status(ctx);
metrics,prom:=a.Monitor.Summary(ctx);
services:=[]map[string]any{{"name":"Docker Engine","available":agentErr==nil&&status.Docker},{"name":"Nginx","available":agentErr==nil&&status.Nginx},{"name":"Certbot","available":agentErr==nil&&status.Certbot},{"name":"Prometheus","available":prom},{"name":"Agente SOR","available":agentErr==nil&&status.Connected}};
httpx.JSON(w,200,map[string]any{"server":s,"metrics":metrics,"services":services,"sites":func()[]siteView{views:=[]siteView{};
for _,s:=range sites{views=append(views,a.view(s))};
return views}(),"recent_activity":audit,"observed_at":time.Now().UTC().Format(time.RFC3339)})}

func(a *App)history(w http.ResponseWriter,r *http.Request){period:=r.URL.Query().Get("range");
if period!="1h"&&period!="6h"&&period!="24h"{invalid(w,r);
return};
out,e:=a.Monitor.History(r.Context(),period);
if e!=nil{fail(w,r,e);
return};
httpx.JSON(w,200,out)}
