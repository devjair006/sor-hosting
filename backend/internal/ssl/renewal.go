package ssl

import (
	"errors"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/domainname"
)

func Renewal(siteID, host, certName string) (agentapi.DomainInput, error) {
	normalized, e := domainname.Normalize(host)
	if e != nil || certName != "sor-"+normalized || !agentapi.OpaqueID.MatchString(siteID) {
		return agentapi.DomainInput{}, errors.New("unregistered certificate")
	}
	return agentapi.DomainInput{SiteID: siteID, Hostname: normalized}, nil
}
