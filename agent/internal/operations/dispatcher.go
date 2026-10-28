package operations
import("context";
"errors";"log/slog";
"time";
"sor.hosting/agent/internal/backups";
"sor.hosting/agent/internal/docker";
"sor.hosting/agent/internal/journal";
"sor.hosting/agent/internal/wordpress";
"sor.hosting/shared/agentapi")
type Dispatcher struct{Store *journal.Store;
Docker *docker.Manager;
WP *wordpress.Manager;
Backups *backups.Manager}
func(d *Dispatcher)Run(ctx context.Context)error{if e:=d.Store.Q.InterruptRunning(ctx);
e!=nil{return e};
tick:=time.NewTicker(time.Second);
defer tick.Stop();
for{select{case <-ctx.Done():return ctx.Err();
case <-tick.C:if e:=d.WP.Nginx.Recover(ctx);
e!=nil{continue};
if e:=d.Backups.Recover(ctx);
e!=nil{continue};
if e:=d.reconcile(ctx);e!=nil{continue};rows,e:=d.Store.Queued(ctx);
if e!=nil{continue};
if len(rows)>0{d.execute(ctx,rows[0])}}}}
func(d *Dispatcher)execute(parent context.Context,r agentapi.OperationRequest){ctx,cancel:=context.WithTimeout(parent,20*time.Minute);
defer cancel();
if e:=d.Store.Update(ctx,r.ID,"running","Preparando operación",nil,"");
e!=nil{return};
checkpoint:=func(phase string)error{return d.Store.Update(ctx,r.ID,"running",phase,nil,"")};
var result any;
var err error;
switch r.Kind{case "container.start","container.stop","container.restart":var p agentapi.ContainerInput;
err=agentapi.Decode(r.Payload,&p);
if err==nil{err=d.Docker.Action(ctx,r.ResourceID,p.Action)};
result=map[string]string{"id":r.ResourceID};
case "site.install":result,err=d.WP.Install(ctx,r,checkpoint);
case "site.adopt":result,err=d.WP.Adopt(ctx,r);
case "domain.configure","certificate.issue","certificate.renew":var p agentapi.DomainInput;
err=agentapi.Decode(r.Payload,&p);
if err==nil{err=checkpoint("Comprobando DNS y certificado")};
if err==nil{result,err=d.WP.ConfigureDomain(ctx,r.ResourceID,p.SiteID,p.Hostname,r.Kind=="certificate.renew")};
case "backup.create","backup.restore":var p agentapi.BackupInput;
err=agentapi.Decode(r.Payload,&p);
if err==nil&&!p.ConfirmMaintenance{err=errors.New("maintenance confirmation required")};
if err==nil{err=checkpoint("Respaldando archivos y base de datos")};
if err==nil{if r.Kind=="backup.create"{if p.SiteID!=r.ResourceID{err=errors.New("site ID mismatch")}else{result,err=d.Backups.Create(ctx,p.SiteID,r.ID)}}else{if p.BackupID!=r.ResourceID{err=errors.New("backup ID mismatch")}else{_ = checkpoint("Validando respaldo y preparando restauración");
result,err=d.Backups.Restore(ctx,p.BackupID,p.SiteID,p.ConfirmDomain,r.ID)}}};
default:err=errors.New("unsupported operation")};
finishCtx,finishCancel:=context.WithTimeout(context.Background(),10*time.Second);
defer finishCancel();
state,phase,code:="succeeded","Operación completada","";
if err==nil{if site,ok:=result.(agentapi.SiteResources);ok{if site.State=="pending_dns"{phase="WordPress instalado; falta completar DNS y SSL"}else if site.State=="pending_ssl"{phase="WordPress instalado; certificado SSL pendiente"}}};if err!=nil{state="failed";
code,phase=publicFailure(err);
if ctx.Err()!=nil||err.Error()=="RECOVERY_REQUIRED"{state="interrupted";
phase="Revisión requerida antes de volver a intentar";
code="RECOVERY_REQUIRED"}};
if err!=nil{slog.Warn("operation did not complete","id",r.ID,"kind",r.Kind,"code",code)};_ = d.Store.Update(finishCtx,r.ID,state,phase,result,code)}
