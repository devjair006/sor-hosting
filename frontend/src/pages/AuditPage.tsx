import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../services/api";
import type { Page, AuditEvent } from "../services/types";
import { PageHeader } from "../components/layout/AppShell";
import { Card, CardContent } from "../components/ui/card";
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
import { date } from "../lib/utils";
export function AuditPage() {
  const [cursors, setCursors] = useState([""]);
  const cursor = cursors.at(-1) ?? "";
  const q = useQuery({
    queryKey: ["audit", cursor],
    queryFn: ({ signal }) =>
      api<Page<AuditEvent>>("/audit?cursor=" + encodeURIComponent(cursor), {
        signal,
      }),
  });
  return (
    <>
      <PageHeader
        title="Auditoría"
        description="Historial de operaciones, responsables y resultados."
        action={false}
      />
      <Card>
        {q.isLoading ? (
          <Loading />
        ) : q.error ? (
          <ErrorState error={q.error} retry={q.refetch} />
        ) : q.data?.items.length ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>FECHA</TableHead>
                <TableHead>ACCIÓN</TableHead>
                <TableHead>RECURSO</TableHead>
                <TableHead>RESPONSABLE</TableHead>
                <TableHead>RESULTADO</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {q.data.items.map((a) => (
                <TableRow key={a.id}>
                  <TableCell>{date(a.created_at)}</TableCell>
                  <TableCell>{a.action}</TableCell>
                  <TableCell>
                    <code>{a.resource_id.slice(0, 12)}</code>
                  </TableCell>
                  <TableCell>
                    <code>{a.user_id.slice(0, 12)}</code>
                  </TableCell>
                  <TableCell>
                    <Status value={a.outcome} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <Empty title="Sin actividad registrada" />
        )}
        <CardContent>
          <div className="dialog-actions">
            <Button
              variant="outline"
              disabled={cursors.length === 1}
              onClick={() => setCursors((v) => v.slice(0, -1))}
            >
              Anterior
            </Button>
            <Button
              variant="outline"
              disabled={!q.data?.next_cursor}
              onClick={() => {
                const next = q.data?.next_cursor;
                if (next) setCursors((v) => [...v, next]);
              }}
            >
              Siguiente
            </Button>
          </div>
        </CardContent>
      </Card>
    </>
  );
}
