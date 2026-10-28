package monitoring
import("sync";
"time")
type observation struct{Healthy bool;
At time.Time}
type Checker struct{mu sync.Mutex;
values map[string]observation}
func NewChecker()*Checker{return &Checker{values:map[string]observation{}}}
func(c *Checker)Record(id string,healthy bool){c.mu.Lock();
defer c.mu.Unlock();
c.values[id]=observation{healthy,time.Now()}}
func(c *Checker)Result(id string)*bool{c.mu.Lock();
defer c.mu.Unlock();
v,ok:=c.values[id];
if !ok||time.Since(v.At)>2*time.Minute{return nil};
out:=v.Healthy;
return &out}
