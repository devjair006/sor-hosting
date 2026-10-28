package httpapi
import("context";
"sync";
"time";
"sor.hosting/backend/internal/database/sqlc";
"sor.hosting/backend/internal/monitoring")
type siteView struct{sqlc.Site;
Availability *bool `json:"availability"`}
func(a *App)view(s sqlc.Site)siteView{return siteView{Site:s,Availability:a.Health.Result(s.ID)}}
func(a *App)MonitorSites(ctx context.Context){tick:=time.NewTicker(30*time.Second);
defer tick.Stop();
for{select{case <-ctx.Done():return;
case <-tick.C:rows,e:=a.Q.ListSites(ctx);
if e!=nil{continue};
var wg sync.WaitGroup;
sem:=make(chan struct{},4);
for _,s:=range rows{if s.State!="active"{continue};
select{case sem<-struct{}{}:case <-ctx.Done():return};
wg.Add(1);
go func(s sqlc.Site){defer wg.Done();
defer func(){<-sem}();
a.Health.Record(s.ID,monitoring.Probe(ctx,s.Domain))}(s)};
wg.Wait()}}}
