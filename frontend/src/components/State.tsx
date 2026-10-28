import type { ReactNode } from "react";
import { AlertCircle, Inbox, LoaderCircle } from "lucide-react";
import { Button } from "./ui/button";
import { Badge } from "./ui/badge";
const labels: Record<string, string> = {
  active: "Configurado",
  online: "En línea",
  offline: "Fuera de línea",
  running: "En ejecución",
  exited: "Detenido",
  created: "Creado",
  paused: "Pausado",
  pending_dns: "Pendiente de DNS",
  pending_ssl: "SSL pendiente",
  provisioning: "Instalando",
  failed: "Falló",
  restoring: "Restaurando",
  ready: "Disponible",
  queued: "Pendiente",
  succeeded: "Completado",
  interrupted: "Revisión requerida",
  unknown: "Sin verificar",
  matched: "DNS correcto",
  pending: "Pendiente",
};
export function Status({ value }: { value: string }) {
  return (
    <Badge
      className={
        [
          "failed",
          "interrupted",
          "exited",
          "offline",
          "pending_dns",
          "pending_ssl",
          "unknown",
          "pending",
        ].includes(value)
          ? "badge-warning"
          : ""
      }
    >
      <span aria-hidden="true" className="status-dot" />
      {labels[value] ?? value}
    </Badge>
  );
}
export function Empty({
  title = "Todavía no hay registros",
  children,
}: {
  title?: string;
  children?: ReactNode;
}) {
  return (
    <div className="empty">
      <Inbox aria-hidden="true" size={30} />
      <h3>{title}</h3>
      {children && <p>{children}</p>}
    </div>
  );
}
export function Loading() {
  return (
    <div role="status" className="empty">
      <LoaderCircle className="spin" aria-hidden="true" />
      Cargando información…
    </div>
  );
}
export function ErrorState({
  error,
  retry,
}: {
  error?: Error | null;
  retry?: () => unknown;
}) {
  return (
    <div role="alert" className="error-state">
      <AlertCircle aria-hidden="true" />
      <div>
        <strong>No se pudo obtener la información</strong>
        <p>{error?.message ?? "Comprueba la conexión con el servidor."}</p>
      </div>
      {retry && (
        <Button variant="outline" onClick={() => void retry()}>
          Reintentar
        </Button>
      )}
    </div>
  );
}
