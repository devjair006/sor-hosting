import { useState } from "react";
import { useOperation } from "../hooks/useOperation";
import { OperationFeedback } from "../components/OperationFeedback";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "../components/ui/dialog";
import { Input } from "../components/ui/input";
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
  const [deleting, setDeleting] = useState(false);
  const [confirmation, setConfirmation] = useState("");
  const [keepBackups, setKeepBackups] = useState(true);
  const op = useOperation();
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
  const readOnly = d.site.management === "external";
  const present = !d.site.lifecycle || d.site.lifecycle === "present";
  return (
    <>
      <PageHeader
        title={d.site.name}
        description={d.site.domain}
        action={false}
      />
      <div className="detail-actions">
        <Status
          value={present ? d.site.state : (d.site.lifecycle ?? d.site.state)}
        />
        <span className="muted">
          {readOnly ? "Externo · solo lectura" : "Administrado por SOR"}
        </span>
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
        {!readOnly && present && (
          <Button variant="outline" asChild>
            <Link to={"/backups?site=" + id}>
              <Archive size={16} />
              Crear backup
            </Link>
          </Button>
        )}
        {d.site.can_delete && (
          <Button
            variant="destructive"
            onClick={() => {
              setConfirmation("");
              setKeepBackups(true);
              setDeleting(true);
            }}
          >
            Eliminar sitio
          </Button>
        )}
      </div>
      {readOnly && (
        <p className="info-note">
          Registro externo de solo lectura. No permite operar contenedores,
          cambiar bases compartidas ni crear o restaurar backups.
        </p>
      )}
      <OperationFeedback {...op} />
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent>
          <DialogTitle>Eliminar sitio WordPress</DialogTitle>
          <DialogDescription>
            Se verificará la propiedad de todos los recursos y se creará un
            respaldo de seguridad. Una vez iniciado el borrado de volúmenes, la
            recuperación continúa hasta finalizar; los datos borrados no tienen
            rollback directo.
          </DialogDescription>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (confirmation !== d.site.domain) return;
              void op
                .run("/sites/" + id + "/delete", {
                  confirm_domain: confirmation,
                  keep_backups: keepBackups,
                })
                .then((job) => {
                  if (job) setDeleting(false);
                });
            }}
          >
            <div className="field">
              <label htmlFor="delete-domain">
                Escribe el dominio completo: {d.site.domain}
              </label>
              <Input
                id="delete-domain"
                autoComplete="off"
                value={confirmation}
                onChange={(e) => setConfirmation(e.target.value)}
                required
              />
            </div>
            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={keepBackups}
                onChange={(e) => setKeepBackups(e.target.checked)}
              />{" "}
              Conservar backups para descarga y recuperación manual
            </label>
            {op.error && (
              <p role="alert" className="field-error">
                {op.error}
              </p>
            )}
            <Button
              type="submit"
              variant="destructive"
              disabled={op.busy || confirmation !== d.site.domain}
            >
              Confirmar eliminación
            </Button>
          </form>
        </DialogContent>
      </Dialog>
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
            {!readOnly && present && (
              <Link to="/containers" className="text-link">
                Administrar contenedores →
              </Link>
            )}
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
            {!readOnly && present && (
              <Link to="/domains" className="text-link">
                Dominios y SSL →
              </Link>
            )}
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
