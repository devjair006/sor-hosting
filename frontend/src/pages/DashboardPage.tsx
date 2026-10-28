import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  Cpu,
  MemoryStick,
  HardDrive,
  Network,
  Check,
  Pause,
  Play,
  ArrowUpRight,
  Server,
} from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";
import { api } from "../services/api";
import type { Dashboard, Monitoring } from "../services/types";
import { bytes, date } from "../lib/utils";
import { PageHeader } from "../components/layout/AppShell";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "../components/ui/card";
import { Button } from "../components/ui/button";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "../components/ui/table";
import { Status, Empty, ErrorState, Loading } from "../components/State";
export function DashboardPage() {
  const [range, setRange] = useState("6h");
  const [paused, setPaused] = useState(false);
  const q = useQuery({
    queryKey: ["dashboard"],
    queryFn: ({ signal }) => api<Dashboard>("/dashboard", { signal }),
    refetchInterval: paused ? false : 30000,
  });
  const monitoring = useQuery({
    queryKey: ["monitoring", range],
    queryFn: ({ signal }) =>
      api<Monitoring>("/monitoring?range=" + range, { signal }),
    refetchInterval: paused ? false : 30000,
  });
  const d = q.data;
  const m = d?.metrics;
  const series = monitoring.data?.series ?? [];
  const points = new Map<
    number,
    { timestamp: number; cpu?: number | null; memory?: number | null }
  >();
  for (const s of series) {
    if (!["cpu", "memory"].includes(s.name)) continue;
    for (const p of s.points)
      points.set(p.timestamp, {
        ...points.get(p.timestamp),
        timestamp: p.timestamp,
        [s.name]: p.value,
      });
  }
  const chart = [...points.values()].sort((a, b) => a.timestamp - b.timestamp);
  const values = [
    {
      name: "CPU",
      icon: Cpu,
      value: m?.cpu != null ? m.cpu.toFixed(1) + " %" : "—",
      note: m?.cpu != null ? "Uso del procesador" : "Sin datos de Prometheus",
      pct: m?.cpu,
    },
    {
      name: "MEMORIA",
      icon: MemoryStick,
      value: bytes(m?.memory_used),
      note:
        m?.memory_total != null
          ? "de " + bytes(m.memory_total)
          : "Sin datos de Prometheus",
      pct: m?.memory_pct,
    },
    {
      name: "ALMACENAMIENTO",
      icon: HardDrive,
      value: bytes(m?.disk_used),
      note:
        m?.disk_total != null
          ? "de " + bytes(m.disk_total)
          : "Sin datos de Prometheus",
      pct: m?.disk_total
        ? ((m.disk_used ?? 0) / m.disk_total) * 100
        : undefined,
    },
    {
      name: "TRÁFICO",
      icon: Network,
      value: m?.traffic != null ? bytes(m.traffic) + "/s" : "—",
      note: "Entrada y salida",
      pct: undefined,
    },
  ];
  return (
    <>
      <PageHeader
        eyebrow="TU INFRAESTRUCTURA, EN ORDEN"
        title="Resumen del servidor"
        description="Recursos, sitios y operaciones desde un solo lugar."
      />
      {q.error && <ErrorState error={q.error} retry={q.refetch} />}
      <section className="metrics-grid" aria-label="Recursos del servidor">
        {values.map(({ name, icon: Icon, value, note, pct }) => (
          <Card key={name} className="metric-card">
            <div className="metric-label">
              {name}
              <Icon size={17} aria-hidden="true" />
            </div>
            <strong className="metric-value">{value}</strong>
            <div className="meter" aria-hidden="true">
              <span
                style={{ width: `${Math.max(0, Math.min(pct ?? 0, 100))}%` }}
              />
            </div>
            <p>{note}</p>
          </Card>
        ))}
      </section>
      <div className="dashboard-charts">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Rendimiento</CardTitle>
              <p className="muted">CPU y memoria del servidor</p>
            </div>
            <div className="chart-controls">
              <label className="sr-only" htmlFor="range">
                Intervalo de métricas
              </label>
              <select
                id="range"
                value={range}
                onChange={(e) => setRange(e.target.value)}
              >
                {["1h", "6h", "24h"].map((v) => (
                  <option key={v} value={v}>
                    {v.replace("h", " h")}
                  </option>
                ))}
              </select>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPaused(!paused)}
              >
                {paused ? <Play size={14} /> : <Pause size={14} />}
                <span>{paused ? "Reanudar" : "Pausar"}</span>
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {monitoring.isLoading ? (
              <Loading />
            ) : monitoring.error ? (
              <ErrorState error={monitoring.error} retry={monitoring.refetch} />
            ) : chart.length ? (
              <>
                <div
                  className="chart"
                  role="img"
                  aria-label="Historial de uso de CPU y memoria, en porcentaje"
                >
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart
                      data={chart}
                      margin={{ top: 10, right: 12, left: -22, bottom: 0 }}
                      accessibilityLayer
                    >
                      <CartesianGrid
                        strokeDasharray="0"
                        vertical={false}
                        stroke="var(--border)"
                      />
                      <XAxis
                        dataKey="timestamp"
                        tickFormatter={(v) =>
                          new Date(v * 1000).toLocaleTimeString("es", {
                            hour: "2-digit",
                            minute: "2-digit",
                          })
                        }
                        tickLine={false}
                        axisLine={false}
                        minTickGap={35}
                        fontSize={11}
                      />
                      <YAxis
                        unit=" %"
                        domain={[0, 100]}
                        tickLine={false}
                        axisLine={false}
                        fontSize={11}
                      />
                      <Tooltip
                        labelFormatter={(v) =>
                          new Date(Number(v) * 1000).toLocaleTimeString("es")
                        }
                        formatter={(v) => [Number(v).toFixed(1) + " %"]}
                      />
                      <Legend />
                      <Line
                        dataKey="cpu"
                        name="CPU"
                        stroke="var(--primary)"
                        dot={false}
                        strokeWidth={2}
                        isAnimationActive={false}
                      />
                      <Line
                        dataKey="memory"
                        name="Memoria"
                        stroke="var(--chart-secondary)"
                        strokeDasharray="5 3"
                        dot={false}
                        strokeWidth={2}
                        isAnimationActive={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
                <p className="chart-updated">
                  {paused ? "Actualización pausada · " : ""}Último dato:{" "}
                  {date(monitoring.data?.observed_at)}
                </p>
              </>
            ) : (
              <Empty title="Aún no hay métricas">
                Comprueba la conexión y la instancia de Prometheus.
              </Empty>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Servicios del servidor</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="service-list">
              {(
                d?.services ?? [
                  { name: "Agente SOR", available: false },
                  { name: "Docker Engine", available: false },
                  { name: "Prometheus", available: false },
                ]
              ).map((s) => (
                <div className="service-row" key={s.name}>
                  <span className="service-icon">
                    {s.available ? <Check size={17} /> : <Server size={17} />}
                  </span>
                  <strong>{s.name}</strong>
                  <span
                    className={"badge " + (!s.available ? "badge-warning" : "")}
                  >
                    {s.available ? "Activo" : "Sin conexión"}
                  </span>
                </div>
              ))}
            </div>
            <p className="muted small">
              Disponibilidad de servicios e integraciones.
            </p>
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Sitios WordPress</CardTitle>
          <Link className="text-link" to="/sites">
            Ver todos <span aria-hidden="true">→</span>
          </Link>
        </CardHeader>
        {d?.sites.length ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>SITIO</TableHead>
                <TableHead>ESTADO</TableHead>
                <TableHead>DOMINIO</TableHead>
                <TableHead>ACCESO</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {d.sites.slice(0, 5).map((s) => (
                <TableRow key={s.id}>
                  <TableCell>
                    <Link className="site-name" to={"/sites/" + s.id}>
                      <span className="site-avatar">
                        {s.name[0]?.toUpperCase()}
                      </span>
                      <strong>{s.name}</strong>
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Status
                      value={
                        s.availability === true
                          ? "online"
                          : s.availability === false
                            ? "offline"
                            : s.state
                      }
                    />
                  </TableCell>
                  <TableCell>
                    <a
                      href={"https://" + s.domain}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {s.domain}
                    </a>
                  </TableCell>
                  <TableCell>
                    <a
                      className="text-link"
                      href={"https://" + s.domain + "/wp-admin"}
                      target="_blank"
                      rel="noreferrer"
                    >
                      wp-admin <ArrowUpRight size={14} aria-hidden="true" />
                    </a>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <Empty title="Tu primer sitio empieza aquí">
            Crea un WordPress o importa un sitio autorizado desde Sitios
            WordPress.
          </Empty>
        )}
      </Card>
      <section className="activity">
        <p className="eyebrow">ACTIVIDAD RECIENTE</p>
        {d?.recent_activity.length ? (
          <ul>
            {d.recent_activity.slice(0, 4).map((a) => (
              <li key={a.id}>
                <span className="status-dot" />
                <strong>{a.action}</strong>
                <Status value={a.outcome} />
                <time>{date(a.created_at)}</time>
              </li>
            ))}
          </ul>
        ) : (
          <p className="muted">Las operaciones del panel aparecerán aquí.</p>
        )}
      </section>
    </>
  );
}
