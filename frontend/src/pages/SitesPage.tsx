import { Pagination } from "../components/Pagination";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Search, ArrowUpRight, Download } from "lucide-react";
import { api } from "../services/api";
import type { Page, Site } from "../services/types";
import { PageHeader } from "../components/layout/AppShell";
import { Card, CardHeader } from "../components/ui/card";
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
import { Loading, Empty, ErrorState, Status } from "../components/State";
import { useOperation } from "../hooks/useOperation";
import { OperationFeedback } from "../components/OperationFeedback";
export function SitesPage() {
  const [cursor, setCursor] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [open, setOpen] = useState(false);
  const q = useQuery({
    queryKey: ["sites", cursor, search, status],
    queryFn: ({ signal }) =>
      api<Page<Site>>(
        "/sites?cursor=" +
          encodeURIComponent(cursor) +
          "&q=" +
          encodeURIComponent(search) +
          "&status=" +
          encodeURIComponent(status),
        {
          signal,
        },
      ),
  });
  const op = useOperation();
  const rows =
    q.data?.items.filter(
      (s) =>
        (s.name + " " + s.domain)
          .toLowerCase()
          .includes(search.toLowerCase()) &&
        (!status || s.state === status),
    ) ?? [];
  return (
    <>
      <PageHeader
        title="Sitios WordPress"
        description="Tus sitios, sus dominios y los accesos de administración."
      />
      <Card>
        <CardHeader>
          <div className="search-field">
            <Search size={17} aria-hidden="true" />
            <Input
              aria-label="Buscar sitios"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setCursor("");
              }}
              placeholder="Buscar por nombre o dominio"
            />
          </div>
          <div className="row-actions">
            <select
              aria-label="Filtrar por estado"
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setCursor("");
              }}
            >
              <option value="">Todos los estados</option>
              <option value="active">Configurados</option>
              <option value="pending_dns">Pendiente de DNS</option>
              <option value="failed">Falló</option>
            </select>
            <Button variant="outline" onClick={() => setOpen(true)}>
              <Download size={16} aria-hidden="true" />
              Importar existente
            </Button>
          </div>
        </CardHeader>
        {q.isLoading ? (
          <Loading />
        ) : q.error ? (
          <ErrorState error={q.error} retry={q.refetch} />
        ) : rows.length ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>SITIO</TableHead>
                <TableHead>DOMINIO</TableHead>
                <TableHead>ESTADO</TableHead>
                <TableHead>ACCESO</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((s) => (
                <TableRow key={s.id}>
                  <TableCell>
                    <Link to={"/sites/" + s.id} className="site-name">
                      <span className="site-avatar">
                        {s.name[0]?.toUpperCase()}
                      </span>
                      <strong>{s.name}</strong>
                    </Link>
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
                      href={"https://" + s.domain + "/wp-admin"}
                      target="_blank"
                      rel="noreferrer"
                      className="text-link"
                    >
                      wp-admin
                      <ArrowUpRight size={14} aria-hidden="true" />
                    </a>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <Empty
            title={
              search
                ? "No encontramos coincidencias"
                : "Todavía no tienes sitios"
            }
          >
            Crea tu primer WordPress o importa uno autorizado por el agente.
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
          <DialogTitle>Importar WordPress existente</DialogTitle>
          <DialogDescription>
            El administrador del VPS debe autorizar previamente los IDs exactos
            y la configuración del sitio en el agente. Esta operación no
            reinicia los contenedores.
          </DialogDescription>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const d = new FormData(e.currentTarget);
              void op
                .run("/sites/adopt", {
                  name: d.get("name"),
                  domain: d.get("domain"),
                  web_container_id: d.get("web"),
                  database_container_id: d.get("db"),
                  database_secret_ref: d.get("secret"),
                  writer_container_ids: [],
                })
                .then((job) => {
                  if (job) setOpen(false);
                });
            }}
          >
            {[
              ["name", "Nombre del sitio"],
              ["domain", "Dominio"],
              ["web", "ID completo del contenedor WordPress"],
              ["db", "ID completo del contenedor de base de datos"],
              ["secret", "Referencia de secretos autorizada"],
            ].map(([name, label]) => (
              <div className="field" key={name}>
                <label htmlFor={"adopt-" + name}>{label}</label>
                <Input
                  id={"adopt-" + name}
                  name={name}
                  required
                  maxLength={253}
                />
              </div>
            ))}
            {op.error && (
              <p role="alert" className="field-error">
                {op.error}
              </p>
            )}
            <Button type="submit" disabled={op.busy}>
              Importar sitio
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
