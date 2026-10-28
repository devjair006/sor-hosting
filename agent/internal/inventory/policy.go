package inventory

import "sor.hosting/shared/agentapi"

func Authorized(site agentapi.SiteResources, id string, protected bool) bool {
	if protected || !site.Mutable() {
		return false
	}
	if id == site.WebID || id == site.DBID {
		return true
	}
	for _, v := range site.Writers {
		if v == id {
			return true
		}
	}
	return false
}
