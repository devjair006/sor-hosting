-- name: CreateUser :exec
INSERT INTO users(id,email,password_hash,role,created_at) VALUES(?,?,?,?,?);
-- name: UserByEmail :one
SELECT * FROM users WHERE email=?;
-- name: UserByID :one
SELECT * FROM users WHERE id=?;
-- name: ChangePassword :exec
UPDATE users SET password_hash=? WHERE id=?;
-- name: CreateSession :exec
INSERT INTO sessions(token_hash,user_id,created_at,last_seen_at,expires_at) VALUES(?,?,?,?,?);
-- name: Session :one
SELECT * FROM sessions WHERE token_hash=?;
-- name: TouchSession :exec
UPDATE sessions SET last_seen_at=? WHERE token_hash=?;
-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash=?;
-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id=?;
-- name: PurgeSessions :exec
DELETE FROM sessions WHERE expires_at<? OR last_seen_at<?;
-- name: ListSites :many
SELECT * FROM sites ORDER BY created_at DESC,id DESC;
-- name: Site :one
SELECT * FROM sites WHERE id=?;
-- name: CreateSite :exec
INSERT INTO sites(id,server_id,name,domain,state,metadata,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?);
-- name: UpdateSite :exec
UPDATE sites SET state=?,metadata=?,updated_at=? WHERE id=?;
-- name: ListDomains :many
SELECT * FROM domains ORDER BY hostname;
-- name: Domain :one
SELECT * FROM domains WHERE id=?;
-- name: CreateDomain :exec
INSERT INTO domains(id,site_id,hostname,dns_state,metadata,created_at) VALUES(?,?,?,?,?,?);
-- name: UpdateDomain :exec
UPDATE domains SET dns_state=?,metadata=? WHERE id=?;
-- name: ListCertificates :many
SELECT * FROM certificates ORDER BY expires_at;
-- name: Certificate :one
SELECT * FROM certificates WHERE id=?;
-- name: SaveCertificate :exec
INSERT INTO certificates(id,domain_id,cert_name,expires_at,metadata) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET expires_at=excluded.expires_at,metadata=excluded.metadata;
-- name: ListBackups :many
SELECT * FROM backups ORDER BY created_at DESC,id DESC;
-- name: Backup :one
SELECT * FROM backups WHERE id=?;
-- name: SaveBackup :exec
INSERT INTO backups(id,site_id,state,size_bytes,checksum,metadata,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING;
-- name: CreateJob :exec
INSERT INTO jobs(id,user_id,kind,resource_id,state,phase,payload,idem_key,digest,result,error_code,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: Job :one
SELECT * FROM jobs WHERE id=?;
-- name: JobByKey :one
SELECT * FROM jobs WHERE user_id=? AND idem_key=?;
-- name: PendingJobs :many
SELECT * FROM jobs WHERE state IN ('queued','running') ORDER BY created_at,id;
-- name: UpdateJob :exec
UPDATE jobs SET state=?,phase=?,payload=?,result=?,error_code=?,updated_at=? WHERE id=?;
-- name: InsertAudit :exec
INSERT INTO audit_events(id,user_id,action,resource_id,outcome,created_at) VALUES(?,?,?,?,?,?);
-- name: ListAudit :many
SELECT * FROM audit_events ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?;
-- name: Server :one
SELECT * FROM servers WHERE id='local';
-- name: RenameServer :exec
UPDATE servers SET name=? WHERE id='local';

-- name: DeleteSiteContainers :exec
DELETE FROM site_containers WHERE site_id=?;
-- name: BindContainer :exec
INSERT INTO site_containers(site_id,container_id,role) VALUES(?,?,?) ON CONFLICT(container_id) DO UPDATE SET site_id=excluded.site_id,role=excluded.role;
