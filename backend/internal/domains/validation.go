package domains
import("errors";
"sor.hosting/shared/agentapi";
"sor.hosting/shared/domainname")
func Validate(in *agentapi.DomainInput)error{host,e:=domainname.Normalize(in.Hostname);
if e!=nil||!agentapi.OpaqueID.MatchString(in.SiteID){return errors.New("invalid domain parameters")};
in.Hostname=host;
return nil}
