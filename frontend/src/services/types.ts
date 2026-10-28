export type User = { id: string; email: string; role: "admin" };
export type Job = {
  id: string;
  kind: string;
  resource_id: string;
  state: "queued" | "running" | "succeeded" | "failed" | "interrupted";
  phase: string;
  error_code: string;
  created_at: string;
  updated_at: string;
};
export type Page<T> = { items: T[]; next_cursor: string | null };
export type Site = {
  availability?: boolean | null;
  id: string;
  name: string;
  domain: string;
  state: string;
  created_at: string;
  updated_at: string;
  metadata?: unknown;
};
export type Container = {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  managed: boolean;
  can_manage: boolean;
  can_read_logs: boolean;
  protected: boolean;
  site_id: string;
  cpu_pct: number | null;
  memory_bytes: number | null;
};
export type Domain = {
  id: string;
  site_id: string;
  hostname: string;
  dns_state: string;
  metadata?: string;
};
export type Certificate = {
  id: string;
  domain_id: string;
  cert_name: string;
  expires_at: string;
  metadata?: string;
};
export type Backup = {
  id: string;
  site_id: string;
  state: string;
  size_bytes: number;
  checksum: string;
  created_at: string;
};
export type AuditEvent = {
  id: string;
  user_id: string;
  action: string;
  resource_id: string;
  outcome: string;
  created_at: string;
};
export type Series = {
  name: string;
  unit: string;
  points: { timestamp: number; value: number | null }[];
};
export type Monitoring = {
  series: Series[];
  observed_at: string;
  stale: boolean;
};
export type Dashboard = {
  server: { id: string; name: string };
  metrics: {
    cpu: number | null;
    memory_pct: number | null;
    memory_used: number | null;
    memory_total: number | null;
    disk_used: number | null;
    disk_total: number | null;
    traffic: number | null;
  };
  services: { name: string; available: boolean }[];
  sites: Site[];
  recent_activity: AuditEvent[];
  observed_at: string;
};
