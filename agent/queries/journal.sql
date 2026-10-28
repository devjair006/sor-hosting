-- name: Reserve :execrows
INSERT OR IGNORE INTO operations(id,kind,resource_id,digest,state,phase,request,result,error_code,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?);
-- name: Operation :one
SELECT id,kind,resource_id,digest,state,phase,request,result,error_code,updated_at FROM operations WHERE id=?;
-- name: Pending :many
SELECT id,kind,resource_id,digest,state,phase,request,result,error_code,updated_at FROM operations WHERE state='queued' ORDER BY updated_at,id;
-- name: UpdateOperation :exec
UPDATE operations SET state=?,phase=?,request=?,result=?,error_code=?,updated_at=? WHERE id=?;
-- name: InterruptRunning :exec
UPDATE operations SET state='interrupted',phase=?,request='',error_code='RECOVERY_REQUIRED' WHERE state='running' AND kind NOT IN ('site.delete','site.external');
-- name: SaveSite :exec
INSERT INTO sites(id,metadata) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET metadata=excluded.metadata;
-- name: Site :one
SELECT id,metadata FROM sites WHERE id=?;
-- name: Sites :many
SELECT id,metadata FROM sites;
-- name: SaveDomain :exec
INSERT INTO domains(id,site_id,metadata) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET metadata=excluded.metadata;
-- name: Domain :one
SELECT id,site_id,metadata FROM domains WHERE id=?;
-- name: Domains :many
SELECT id,site_id,metadata FROM domains WHERE site_id=?;
-- name: SaveBackup :exec
INSERT INTO backups(id,site_id,metadata) VALUES(?,?,?) ON CONFLICT(id) DO NOTHING;
-- name: Backup :one
SELECT id,site_id,metadata FROM backups WHERE id=?;
-- name: SaveRecovery :exec
INSERT INTO recovery(id,site_id,metadata) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET metadata=excluded.metadata;
-- name: Recoveries :many
SELECT id,site_id,metadata FROM recovery;
-- name: DeleteRecovery :exec
DELETE FROM recovery WHERE id=?;
-- name: Interrupted :many
SELECT id,kind,resource_id,digest,state,phase,request,result,error_code,updated_at FROM operations WHERE (state='interrupted' AND result IN ('null','{}')) OR (state='running' AND kind IN ('site.delete','site.external'));
