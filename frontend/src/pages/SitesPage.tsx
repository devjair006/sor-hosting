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
  const [registration, setRegistration] = useState<"external" | "adopt">(
    "external",
  );
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
              Registrar existente
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
                <TableHead>TIPO</TableHead>
                <TableHead>ESTADO</TableHead>
                <TableHead>ACCESO</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((s) => (
                <TableRow key={s.id}>
                  <TableCell>
                    <Link to={"/sites/" + s.id} className="site-name">
                      <span className="site-avatar" aria-hidden="true">
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
                    {s.management === "external"
                      ? "Externo · solo lectura"
                      : "Administrado por SOR"}
                  </TableCell>
                  <TableCell>
                    <Status
                      value={
                        s.lifecycle === "deleting"
                          ? "Eliminando"
                          : s.availability === true
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
          <DialogTitle>Registrar WordPress existente</DialogTitle>
          <DialogDescription>
            Registra un sitio como lectura aunque comparta MariaDB. La adopción
            completa sigue disponible para instalaciones compatibles autorizadas
            por el administrador.
          </DialogDescription>
          <div className="field">
            <label htmlFor="registration-mode">Tipo de registro</label>
            <select
              id="registration-mode"
              value={registration}
              onChange={(e) =>
                setRegistration(e.target.value as "external" | "adopt")
              }
            >
              <option value="external">Externo · solo lectura</option>
              <option value="adopt">Adopción completa autorizada</option>
            </select>
          </div>
          <form
            key={registration}
            onSubmit={(e) => {
              e.preventDefault();
              const d = new FormData(e.currentTarget);
              void op
                .run(
                  registration === "external"
                    ? "/sites/external"
                    : "/sites/adopt",
                  {
                    name: d.get("name"),
                    domain: d.get("domain"),
                    web_container_id: d.get("web"),
                    database_container_id: d.get("db"),
                    ...(registration === "adopt"
                      ? {
                          database_secret_ref: d.get("secret"),
                          writer_container_ids: [],
                        }
                      : {}),
                  },
                )
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
            ]
              .filter(([name]) => registration === "adopt" || name !== "secret")
              .map(([name, label]) => (
                <div className="field" key={name}>
                  <label htmlFor={"adopt-" + name}>{label}</label>
                  <Input
                    id={"adopt-" + name}
                    name={name}
                    required={
                      registration === "adopt" ||
                      name === "name" ||
                      name === "domain"
                    }
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
              {registration === "external"
                ? "Registrar en lectura"
                : "Adoptar sitio"}
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
