import { Pagination } from "../components/Pagination";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Play,
  Square,
  RotateCw,
  ScrollText,
  LockKeyhole,
  RefreshCw,
} from "lucide-react";
import { api } from "../services/api";
import type { Page, Container } from "../services/types";
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
import { Status, Empty, ErrorState, Loading } from "../components/State";
import { useOperation } from "../hooks/useOperation";
import { OperationFeedback } from "../components/OperationFeedback";
import { bytes } from "../lib/utils";
export function ContainersPage() {
  const [cursor, setCursor] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<{
    container: Container;
    action: string;
  } | null>(null);
  const [logsTarget, setLogsTarget] = useState<Container | null>(null);
  const q = useQuery({
    queryKey: ["containers", cursor, search],
    queryFn: ({ signal }) =>
      api<Page<Container>>(
        "/containers?cursor=" +
          encodeURIComponent(cursor) +
          "&q=" +
          encodeURIComponent(search),
        {
          signal,
        },
      ),
    refetchInterval: 30000,
  });
  const logs = useQuery({
    queryKey: ["logs", logsTarget?.id],
    queryFn: ({ signal }) =>
      api<{ lines: string[]; truncated: boolean }>(
        "/containers/" + logsTarget?.id + "/logs?tail=200",
        { signal },
      ),
    enabled: !!logsTarget,
    staleTime: 0,
  });
  const op = useOperation();
  const rows =
    q.data?.items.filter((c) =>
      (c.name + c.image).toLowerCase().includes(search.toLowerCase()),
    ) ?? [];
  const actions: Record<string, string> = {
    start: "Iniciar",
    stop: "Detener",
    restart: "Reiniciar",
  };
  return (
    <>
      <PageHeader
        title="Contenedores"
        description="Estado y operaciones de tus servicios Docker autorizados."
      />
      <Card>
        <CardHeader>
          <Input
            className="search-input"
            aria-label="Buscar contenedores"
            placeholder="Buscar contenedor o imagen"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setCursor("");
            }}
          />
          <Button variant="outline" onClick={() => void q.refetch()}>
            <RefreshCw size={15} />
            Actualizar
          </Button>
        </CardHeader>
        {q.isLoading ? (
          <Loading />
        ) : q.error ? (
          <ErrorState error={q.error} retry={q.refetch} />
        ) : rows.length ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>CONTENEDOR</TableHead>
                <TableHead>ESTADO</TableHead>
                <TableHead>CPU / MEMORIA</TableHead>
                <TableHead>OPERACIONES</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((c) => (
                <TableRow key={c.id}>
                  <TableCell>
                    <strong>{c.name}</strong>
                    <small className="block muted">{c.image}</small>
                    <code className="small">{c.id.slice(0, 12)}</code>
                  </TableCell>
                  <TableCell>
                    <Status value={c.state} />
                  </TableCell>
                  <TableCell>
                    {c.cpu_pct != null ? c.cpu_pct.toFixed(1) + " %" : "—"} /{" "}
                    {bytes(c.memory_bytes)}
                  </TableCell>
                  <TableCell>
                    <div className="row-actions">
                      {c.can_manage ? (
                        <>
                          {[
                            ["start", Play],
                            ["stop", Square],
                            ["restart", RotateCw],
                          ].map(([action, Icon]) => {
                            const A = Icon as typeof Play;
                            const a = action as string;
                            return (
                              <Button
                                key={a}
                                variant="outline"
                                size="icon"
                                aria-label={actions[a] + " " + c.name}
                                disabled={
                                  op.busy ||
                                  (a === "start" && c.state === "running") ||
                                  (a !== "start" && c.state !== "running")
                                }
                                onClick={() =>
                                  setSelected({ container: c, action: a })
                                }
                              >
                                <A size={16} />
                              </Button>
                            );
                          })}
                        </>
                      ) : (
                        <span className="muted small">
                          <LockKeyhole size={13} aria-hidden="true" />{" "}
                          {c.protected ? "Protegido" : "Solo lectura"}
                        </span>
                      )}
                      {c.can_read_logs && (
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={"Ver logs de " + c.name}
                          onClick={() => setLogsTarget(c)}
                        >
                          <ScrollText size={17} />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <Empty title="No hay contenedores">
            El inventario aparecerá cuando Docker esté conectado.
          </Empty>
        )}
      </Card>
      <Pagination
        cursor={cursor}
        next={q.data?.next_cursor}
        onChange={setCursor}
      />
      <OperationFeedback {...op} />
      <Dialog open={!!selected} onOpenChange={(v) => !v && setSelected(null)}>
        <DialogContent>
          <DialogTitle>
            {actions[selected?.action ?? "start"]} contenedor
          </DialogTitle>
          <DialogDescription>
            Esta acción afecta a {selected?.container.name}. Detener o reiniciar
            puede interrumpir temporalmente el sitio asociado.
          </DialogDescription>
          <div className="dialog-actions">
            <Button variant="outline" onClick={() => setSelected(null)}>
              Cancelar
            </Button>
            <Button
              variant={selected?.action === "stop" ? "destructive" : "default"}
              disabled={op.busy}
              onClick={() => {
                if (selected)
                  void op
                    .run("/containers/" + selected.container.id + "/actions", {
                      action: selected.action,
                    })
                    .then((j) => {
                      if (j) setSelected(null);
                    });
              }}
            >
              Confirmar
            </Button>
          </div>
          {op.error && (
            <p role="alert" className="field-error">
              {op.error}
            </p>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!logsTarget}
        onOpenChange={(v) => !v && setLogsTarget(null)}
      >
        <DialogContent className="logs-dialog">
          <DialogTitle>Logs · {logsTarget?.name}</DialogTitle>
          <DialogDescription>
            Últimas 200 líneas. La actualización es manual y la salida está
            limitada.
          </DialogDescription>
          <Button
            variant="outline"
            onClick={() => void logs.refetch()}
            disabled={logs.isFetching}
          >
            <RefreshCw size={15} />
            Actualizar logs
          </Button>
          {logs.isLoading ? (
            <Loading />
          ) : logs.error ? (
            <ErrorState error={logs.error} retry={logs.refetch} />
          ) : (
            <pre className="logs" tabIndex={0}>
              {logs.data?.lines.join("\n") || "Sin registros."}
            </pre>
          )}
          {logs.data?.truncated && (
            <p className="muted">La salida alcanzó el límite permitido.</p>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
