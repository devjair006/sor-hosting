module sor.hosting/backend

go 1.26.0

require (
 github.com/go-chi/chi/v5 v5.2.1
 modernc.org/sqlite v1.36.3
 golang.org/x/crypto v0.36.0
 golang.org/x/term v0.30.0
 sor.hosting/shared v0.0.0
)

replace sor.hosting/shared => ../shared
