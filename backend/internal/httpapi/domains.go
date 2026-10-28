package httpapi
import(
"net/http"
"github.com/go-chi/chi/v5"
"sor.hosting/backend/internal/httpx"
"sor.hosting/shared/agentapi"
"sor.hosting/backend/internal/domains"
"sor.hosting/backend/internal/ssl"
)
func(a *App)domains(w http.ResponseWriter,r *http.Request){rows,e:=a.Q.ListDomains(r.Context());
if e!=nil{fail(w,r,e);
return};
page(w,r,rows)}

func(a *App)addDomain(w http.ResponseWriter,r *http.Request){var in agentapi.DomainInput;
if !httpx.Decode(w,r,&in){return};
if domains.Validate(&in)!=nil{invalid(w,r);
return};
if _,e:=a.Q.Site(r.Context(),in.SiteID);
e!=nil{fail(w,r,e);
return};
a.enqueue(w,r,"domain.configure","",in)}

func(a *App)checkDomain(w http.ResponseWriter,r *http.Request){d,e:=a.Q.Domain(r.Context(),chi.URLParam(r,"id"));
if e!=nil{fail(w,r,e);
return};
var body struct{};
if !httpx.Decode(w,r,&body){return};
a.enqueue(w,r,"domain.configure",d.ID,agentapi.DomainInput{SiteID:d.SiteID,Hostname:d.Hostname})}

func(a *App)certificates(w http.ResponseWriter,r *http.Request){rows,e:=a.Q.ListCertificates(r.Context());
if e!=nil{fail(w,r,e);
return};
page(w,r,rows)}

func(a *App)renewCertificate(w http.ResponseWriter,r *http.Request){c,e:=a.Q.Certificate(r.Context(),chi.URLParam(r,"id"));
if e!=nil{fail(w,r,e);
return};
d,e:=a.Q.Domain(r.Context(),c.DomainID);
if e!=nil{fail(w,r,e);
return};
var body struct{};
if !httpx.Decode(w,r,&body){return};
in,e:=ssl.Renewal(d.SiteID,d.Hostname,c.CertName);
if e!=nil{invalid(w,r);
return};
a.enqueue(w,r,"certificate.renew",d.ID,in)}
