package containers
import "sor.hosting/shared/agentapi"
func CanUse(c agentapi.Container,logs bool)bool{if !c.Managed||c.Protected||c.SiteID==""{return false};
if logs{return c.CanReadLogs};
return c.CanManage}
