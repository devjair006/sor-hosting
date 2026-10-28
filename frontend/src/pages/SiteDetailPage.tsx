import { useParams, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, Archive } from "lucide-react";
import { api } from "../services/api";
import type { Site, Container, Domain, Backup } from "../services/types";
import { PageHeader } from "../components/layout/AppShell";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Loading, ErrorState, Status, Empty } from "../components/State";
import { bytes, date } from "../lib/utils";
export function SiteDetailPage() {
  const { id } = useParams();
  const q = useQuery({
    queryKey: ["site", id],
    queryFn: ({ signal }) =>
      api<{
        site: Site;
        containers: Container[];
        domains: Domain[];
        backups: Backup[];
      }>("/sites/" + id, { signal }),
  });
  if (q.isLoading) return <Loading />;
  if (q.error || !q.data)
    return <ErrorState error={q.error} retry={q.refetch} />;
  const d = q.data;
  return (
    <>
      <PageHeader
        title={d.site.name}
        description={d.site.domain}
        action={false}
      />
      <div className="detail-actions">
        <Status value={d.site.state} />
        <Button variant="outline" asChild>
          <a
            href={"https://" + d.site.domain + "/wp-admin"}
            target="_blank"
            rel="noreferrer"
          >
            Abrir wp-admin
            <ExternalLink size={15} />
          </a>
        </Button>
        <Button variant="outline" asChild>
          <Link to={"/backups?site=" + id}>
            <Archive size={16} />
            Crear backup
          </Link>
        </Button>
      </div>
      <div className="detail-grid">
        <Card>
          <CardHeader>
            <CardTitle>Contenedores asociados</CardTitle>
          </CardHeader>
          <CardContent>
            {d.containers.length ? (
              d.containers.map((c) => (
                <div className="detail-row" key={c.id}>
                  <div>
                    <strong>{c.name}</strong>
                    <small>{c.image}</small>
                  </div>
                  <Status value={c.state} />
                </div>
              ))
            ) : (
              <Empty title="Sin contenedores disponibles">
                Comprueba el estado del agente.
              </Empty>
            )}
            <Link to="/containers" className="text-link">
              Administrar contenedores →
            </Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Dominios</CardTitle>
          </CardHeader>
          <CardContent>
            {d.domains.length ? (
              d.domains.map((x) => (
                <div className="detail-row" key={x.id}>
                  <strong>{x.hostname}</strong>
                  <Status value={x.dns_state} />
                </div>
              ))
            ) : (
              <p>{d.site.domain}</p>
            )}
            <Link to="/domains" className="text-link">
              Dominios y SSL →
            </Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Backups recientes</CardTitle>
          </CardHeader>
          <CardContent>
            {d.backups.length ? (
              d.backups.slice(0, 5).map((x) => (
                <div className="detail-row" key={x.id}>
                  <span>{date(x.created_at)}</span>
                  <span>{bytes(x.size_bytes)}</span>
                </div>
              ))
            ) : (
              <Empty title="Sin backups">
                Crea un respaldo antes de realizar cambios importantes.
              </Empty>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Información del sitio</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="definition-list">
              <dt>Creado</dt>
              <dd>{date(d.site.created_at)}</dd>
              <dt>Última actualización</dt>
              <dd>{date(d.site.updated_at)}</dd>
              <dt>Referencia</dt>
              <dd>
                <code>{d.site.id}</code>
              </dd>
            </dl>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
