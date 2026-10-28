package httpx
import("encoding/json";
"net/http";
"github.com/go-chi/chi/v5/middleware";
"sor.hosting/shared/agentapi";
"io")
func JSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");
w.Header().Set("Cache-Control","no-store");
w.WriteHeader(status);
_ = json.NewEncoder(w).Encode(v)}
func Error(w http.ResponseWriter,r *http.Request,status int,code,msg string){JSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":middleware.GetReqID(r.Context())}})}
func Decode(w http.ResponseWriter,r *http.Request,v any)bool{r.Body=http.MaxBytesReader(w,r.Body,65536);
b,e:=io.ReadAll(r.Body);
if e!=nil||agentapi.Decode(b,v)!=nil{Error(w,r,422,"VALIDATION_ERROR","Revisa los datos enviados.");
return false};
return true}
