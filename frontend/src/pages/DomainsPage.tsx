import { Pagination } from "../components/Pagination";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus, ShieldCheck, RefreshCw } from "lucide-react";
import { api, listAll } from "../services/api";
import type { Page, Domain, Certificate, Site } from "../services/types";
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
import { date } from "../lib/utils";
export function DomainsPage() {
  const [cursor, setCursor] = useState("");
  const [open, setOpen] = useState(false);
  const q = useQuery({
    queryKey: ["domains", cursor],
    queryFn: ({ signal }) =>
      api<Page<Domain>>("/domains?cursor=" + encodeURIComponent(cursor), {
        signal,
      }),
  });
  const certs = useQuery({
    queryKey: ["certificates"],
    queryFn: ({ signal }) => listAll<Certificate>("/certificates", signal),
  });
  const sites = useQuery({
    queryKey: ["site-options"],
    queryFn: ({ signal }) => listAll<Site>("/sites", signal),
  });
  const op = useOperation();
  return (
    <>
      <PageHeader
        title="Dominios y SSL"
        description="Comprueba tus dominios y administra sus certificados HTTPS."
        action={false}
      />
      <div className="info-note">
        <ShieldCheck size={20} aria-hidden="true" />
        <p>
          Los registros DNS se crean en tu proveedor. SOR comprueba su destino y
          configura el alojamiento y HTTPS.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Dominios registrados</CardTitle>
          <Button onClick={() => setOpen(true)}>
            <Plus size={16} />
            Agregar dominio
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
                <TableHead>DOMINIO</TableHead>
                <TableHead>DNS</TableHead>
                <TableHead>CERTIFICADO</TableHead>
                <TableHead>VENCIMIENTO</TableHead>
                <TableHead>ACCIONES</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {q.data.items.map((d) => {
                const cert = certs.data?.items.find(
                  (c) => c.domain_id === d.id,
                );
                return (
                  <TableRow key={d.id}>
                    <TableCell>
                      <strong>{d.hostname}</strong>
                    </TableCell>
                    <TableCell>
                      <Status value={d.dns_state} />
                    </TableCell>
                    <TableCell>
                      {cert ? (
                        <span className="text-success">Protegido</span>
                      ) : certs.error ? (
                        <span className="muted">Sin conexión</span>
                      ) : (
                        <span className="muted">Pendiente</span>
                      )}
                    </TableCell>
                    <TableCell>{cert ? date(cert.expires_at) : "—"}</TableCell>
                    <TableCell>
                      <div className="row-actions">
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={op.busy}
                          onClick={() =>
                            void op.run("/domains/" + d.id + "/check", {})
                          }
                        >
                          Comprobar
                        </Button>
                        {cert && (
                          <Button
                            size="sm"
                            variant="ghost"
                            disabled={op.busy}
                            onClick={() =>
                              void op.run(
                                "/certificates/" + cert.id + "/renew",
                                {},
                              )
                            }
                          >
                            <RefreshCw size={14} />
                            Renovar
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        ) : (
          <Empty title="Aún no tienes dominios">
            Los dominios principales aparecen al instalar un sitio. También
            puedes añadir un subdominio.
          </Empty>
        )}
      </Card>
      <Pagination
        cursor={cursor}
        next={q.data?.next_cursor}
        onChange={setCursor}
      />
      <OperationFeedback {...op} />
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogTitle>Agregar dominio</DialogTitle>
          <DialogDescription>
            Asocia un dominio o subdominio a un sitio existente. SOR verificará
            su DNS antes de solicitar SSL.
          </DialogDescription>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const d = new FormData(e.currentTarget);
              void op
                .run("/domains", {
                  site_id: d.get("site"),
                  hostname: d.get("hostname"),
                })
                .then((j) => {
                  if (j) setOpen(false);
                });
            }}
          >
            <div className="field">
              <label htmlFor="domain-site">Sitio</label>
              <select id="domain-site" name="site" required>
                <option value="">Seleccionar sitio</option>
                {sites.data?.items
                  .filter(
                    (s) =>
                      s.management !== "external" &&
                      (!s.lifecycle || s.lifecycle === "present"),
                  )
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} · {s.domain}
                    </option>
                  ))}
              </select>
            </div>
            <div className="field">
              <label htmlFor="hostname">Dominio completo</label>
              <Input
                id="hostname"
                name="hostname"
                placeholder="blog.tu-dominio.com"
                required
                maxLength={253}
              />
            </div>
            {sites.error && (
              <p role="alert" className="field-error">
                No se pudieron obtener los sitios.
              </p>
            )}
            {op.error && (
              <p role="alert" className="field-error">
                {op.error}
              </p>
            )}
            <Button
              type="submit"
              disabled={op.busy || !sites.data?.items.length}
            >
              Agregar y comprobar
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
