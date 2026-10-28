import { Pagination } from "../components/Pagination";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { Plus, Download, History, HardDrive } from "lucide-react";
import { api, listAll } from "../services/api";
import type { Page, Backup, Site } from "../services/types";
import { PageHeader } from "../components/layout/AppShell";
import { Card, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "../components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "../components/ui/dialog";
import { Status, Empty, ErrorState, Loading } from "../components/State";
import { useOperation } from "../hooks/useOperation";
import { OperationFeedback } from "../components/OperationFeedback";
import { bytes, date } from "../lib/utils";
export function BackupsPage() {
  const [cursor, setCursor] = useState("");
  const [params] = useSearchParams();
  const [create, setCreate] = useState(false);
  const [restore, setRestore] = useState<Backup | null>(null);
  const [confirmation, setConfirmation] = useState("");
  const [maintenance, setMaintenance] = useState(false);
  const q = useQuery({
    queryKey: ["backups", cursor],
    queryFn: ({ signal }) =>
      api<Page<Backup>>("/backups?cursor=" + encodeURIComponent(cursor), {
        signal,
      }),
  });
  const sites = useQuery({
    queryKey: ["site-options"],
    queryFn: ({ signal }) => listAll<Site>("/sites", signal),
  });
  const op = useOperation();
  const selected = sites.data?.items.find((s) => s.id === restore?.site_id);
  const name = (id: string) =>
    sites.data?.items.find((s) => s.id === id)?.name ?? id.slice(0, 12);
  return (
    <>
      <PageHeader
        title="Backups"
        description="Respaldos verificables para recuperar tus sitios WordPress."
        action={false}
      />
      <div className="info-note">
        <HardDrive size={22} aria-hidden="true" />
        <p>
          Los respaldos se guardan en este VPS. Descarga una copia externa para
          protegerte ante la pérdida del servidor. Crear o restaurar un backup
          requiere un breve mantenimiento.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Historial de respaldos</CardTitle>
          <Button
            onClick={() => {
              setMaintenance(false);
              setCreate(true);
            }}
          >
            <Plus size={16} />
            Crear backup
          </Button>
        </CardHeader>
        {q.isLoading ? (
          <Loading />
        ) : q.error ? (
          <ErrorState error={q.error} retry={q.refetch} />
        ) : q.data?.items.length ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>SITIO</TableHead>
                <TableHead>FECHA</TableHead>
                <TableHead>TAMAÑO</TableHead>
                <TableHead>ESTADO</TableHead>
                <TableHead>ACCIONES</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {q.data.items.map((b) => (
                <TableRow key={b.id}>
                  <TableCell>
                    <strong>{name(b.site_id)}</strong>
                    <small className="block muted">{b.id.slice(0, 12)}</small>
                  </TableCell>
                  <TableCell>{date(b.created_at)}</TableCell>
                  <TableCell>{bytes(b.size_bytes)}</TableCell>
                  <TableCell>
                    <Status value={b.state} />
                  </TableCell>
                  <TableCell>
                    <div className="row-actions">
                      <Button variant="outline" size="sm" asChild>
                        <a
                          href={"/api/v1/backups/" + b.id + "/download"}
                          download
                        >
                          <Download size={15} />
                          Descargar
                        </a>
                      </Button>
                      <Button
                        variant="ghost"
                        disabled={
                          op.busy ||
                          !sites.data?.items.some(
                            (s) =>
                              s.id === b.site_id &&
                              s.management !== "external" &&
                              (!s.lifecycle || s.lifecycle === "present"),
                          )
                        }
                        size="sm"
                        onClick={() => {
                          setConfirmation("");
                          setMaintenance(false);
                          setRestore(b);
                        }}
                      >
                        <History size={15} />
                        Restaurar
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <Empty title="Todavía no hay backups">
            Crea el primer respaldo de un sitio y comprueba su descarga.
          </Empty>
        )}
      </Card>
      <Pagination
        cursor={cursor}
        next={q.data?.next_cursor}
        onChange={setCursor}
      />
      <OperationFeedback {...op} />
      <Dialog open={create} onOpenChange={setCreate}>
        <DialogContent>
          <DialogTitle>Crear backup</DialogTitle>
          <DialogDescription>
            Se pausarán temporalmente los servicios que escriben en el sitio
            para respaldar sus archivos y base de datos de forma consistente.
          </DialogDescription>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const d = new FormData(e.currentTarget);
              void op
                .run("/sites/" + d.get("site") + "/backups", {
                  confirm_maintenance: maintenance,
                })
                .then((j) => {
                  if (j) setCreate(false);
                });
            }}
          >
            <div className="field">
              <label htmlFor="backup-site">Sitio</label>
              <select
                name="site"
                id="backup-site"
                required
                defaultValue={params.get("site") ?? ""}
              >
                <option value="">Selecciona un sitio</option>
                {sites.data?.items
                  .filter(
                    (s) =>
                      s.management !== "external" &&
                      (!s.lifecycle || s.lifecycle === "present"),
                  )
                  .map((s) => (
                    <option value={s.id} key={s.id}>
                      {s.name} · {s.domain}
                    </option>
                  ))}
              </select>
            </div>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={maintenance}
                onChange={(e) => setMaintenance(e.target.checked)}
                required
              />
              Acepto el periodo de mantenimiento del sitio.
            </label>
            {op.error && (
              <p role="alert" className="field-error">
                {op.error}
              </p>
            )}
            <Button
              type="submit"
              disabled={op.busy || !maintenance || !sites.data?.items.length}
            >
              Crear respaldo
            </Button>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={!!restore} onOpenChange={(v) => !v && setRestore(null)}>
        <DialogContent>
          <DialogTitle>Restaurar sitio</DialogTitle>
          <DialogDescription>
            Se creará primero un respaldo de seguridad. El contenido actual se
            reemplazará por el backup seleccionado; los recursos anteriores se
            conservarán hasta verificar la restauración.
          </DialogDescription>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (restore)
                void op
                  .run("/backups/" + restore.id + "/restore", {
                    confirm_domain: confirmation,
                    confirm_maintenance: maintenance,
                  })
                  .then((j) => {
                    if (j) setRestore(null);
                  });
            }}
          >
            <div className="field">
              <label htmlFor="restore-domain">
                Escribe {selected?.domain ?? "el dominio del sitio"} para
                confirmar
              </label>
              <Input
                id="restore-domain"
                required
                autoComplete="off"
                value={confirmation}
                onChange={(e) => setConfirmation(e.target.value)}
              />
            </div>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={maintenance}
                onChange={(e) => setMaintenance(e.target.checked)}
                required
              />
              Acepto el periodo de mantenimiento.
            </label>
            {op.error && (
              <p role="alert" className="field-error">
                {op.error}
              </p>
            )}
            <Button
              type="submit"
              variant="destructive"
              disabled={
                op.busy ||
                !selected ||
                confirmation !== selected.domain ||
                !maintenance
              }
            >
              Restaurar backup
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
