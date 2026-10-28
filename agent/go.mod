module sor.hosting/agent

go 1.26.0

require (
 github.com/moby/moby/client v0.6.2
 modernc.org/sqlite v1.36.3
 golang.org/x/sys v0.31.0
 sor.hosting/shared v0.0.0
)

replace sor.hosting/shared => ../shared
