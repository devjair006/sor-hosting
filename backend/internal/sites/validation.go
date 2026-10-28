package sites
import("errors";
"net/mail";
"unicode/utf8";
"strings";
"sor.hosting/shared/agentapi";
"sor.hosting/shared/domainname")
func ValidateInstall(in *agentapi.SiteInput)error{host,e:=domainname.Normalize(in.Domain);
email,mailErr:=mail.ParseAddress(in.AdminEmail);
n:=utf8.RuneCountInString(in.AdminPassword);
if e!=nil||mailErr!=nil||email.Address!=in.AdminEmail||len(in.Name)<1||len(in.Name)>80||len(in.WordPressTitle)<1||len(in.WordPressTitle)>200||n<12||n>128||strings.ContainsAny(in.AdminPassword,"\r\n")||in.Resources.MemoryMB<256||in.Resources.MemoryMB>16384||in.Resources.CPULimit<.25||in.Resources.CPULimit>16{return errors.New("invalid WordPress installation parameters")};
in.Domain=host;
return nil}
func ValidateAdoption(in *agentapi.AdoptInput)error{host,e:=domainname.Normalize(in.Domain);
if e!=nil||len(in.Name)<1||len(in.Name)>80||!agentapi.OpaqueID.MatchString(in.WebContainerID)||!agentapi.OpaqueID.MatchString(in.DatabaseContainerID)||!agentapi.OpaqueID.MatchString(in.DatabaseSecretRef){return errors.New("invalid adoption parameters")};
in.Domain=host;
return nil}
