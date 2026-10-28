package httpapi
import(
"net/http"
"strconv"
"github.com/go-chi/chi/v5"
"sor.hosting/backend/internal/database/sqlc"
"sor.hosting/backend/internal/httpx"
"sor.hosting/backend/internal/jobs"
)
func(a *App)job(w http.ResponseWriter,r *http.Request){j,e:=a.Q.Job(r.Context(),chi.URLParam(r,"id"));
if e!=nil{fail(w,r,e);
return};
httpx.JSON(w,200,map[string]any{"job":jobs.Public(j)})}

func(a *App)audit(w http.ResponseWriter,r *http.Request){offset:=int64(0);
if s:=r.URL.Query().Get("cursor");
s!=""{v,e:=strconv.ParseInt(s,10,64);
if e!=nil||v<0||v>10000000{invalid(w,r);
return};
offset=v};
rows,e:=a.Q.ListAudit(r.Context(),sqlc.ListAuditParams{Limit:26,Offset:offset});
if e!=nil{fail(w,r,e);
return};
var next any;
if len(rows)>25{next=strconv.FormatInt(offset+25,10);
rows=rows[:25]};
httpx.JSON(w,200,map[string]any{"items":rows,"next_cursor":next})}
