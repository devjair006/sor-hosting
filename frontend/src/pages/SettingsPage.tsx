import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../services/api";
import { queryClient } from "../lib/queryClient";
import { PageHeader } from "../components/layout/AppShell";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { ErrorState, Loading } from "../components/State";
import { useSession } from "../hooks/useSession";
export function SettingsPage() {
  const q = useQuery({
    queryKey: ["settings"],
    queryFn: ({ signal }) =>
      api<{
        server: { name: string };
        integrations: { agent: boolean; prometheus: boolean };
        backup_destination: string;
      }>("/settings", { signal }),
  });
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const { logout } = useSession();
  async function save(path: string, body: unknown, method = "POST") {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await api(path, { method, body: JSON.stringify(body) });
      if (path === "/auth/password") {
        await logout().catch(() => undefined);
        window.location.assign("/login");
      } else {
        setMessage("Cambios guardados.");
        void queryClient.invalidateQueries({ queryKey: ["settings"] });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo guardar.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeader
        title="Configuración"
        description="Identidad del servidor, conexiones y seguridad de tu cuenta."
        action={false}
      />
      {message && (
        <p role="status" className="info-note">
          {message}
        </p>
      )}
      {error && (
        <p role="alert" className="field-error">
          {error}
        </p>
      )}
      {q.isLoading ? (
        <Loading />
      ) : q.error ? (
        <ErrorState error={q.error} retry={q.refetch} />
      ) : (
        <div className="detail-grid">
          <Card>
            <CardHeader>
              <CardTitle>Servidor</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  const d = new FormData(e.currentTarget);
                  void save(
                    "/settings",
                    { server_name: d.get("name") },
                    "PATCH",
                  );
                }}
              >
                <div className="field">
                  <label htmlFor="server-name">Nombre del servidor</label>
                  <Input
                    id="server-name"
                    name="name"
                    defaultValue={q.data?.server.name}
                    required
                    maxLength={80}
                  />
                </div>
                <Button disabled={busy} type="submit">
                  Guardar nombre
                </Button>
              </form>
              <dl className="definition-list settings-details">
                <dt>Agente</dt>
                <dd>
                  {q.data?.integrations.agent ? "Conectado" : "Sin conexión"}
                </dd>
                <dt>Prometheus</dt>
                <dd>
                  {q.data?.integrations.prometheus
                    ? "Configurado"
                    : "Sin configurar"}
                </dd>
                <dt>Backups</dt>
                <dd>
                  {q.data?.backup_destination ?? "Directorio local del agente"}
                </dd>
              </dl>
              <p className="muted small">
                Las rutas y conexiones sensibles se ajustan en la configuración
                del despliegue.
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Cambiar contraseña</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  const d = new FormData(e.currentTarget);
                  void save("/auth/password", {
                    current_password: d.get("current"),
                    new_password: d.get("new"),
                  });
                }}
              >
                <div className="field">
                  <label htmlFor="current">Contraseña actual</label>
                  <Input
                    id="current"
                    name="current"
                    type="password"
                    autoComplete="current-password"
                    required
                    maxLength={128}
                  />
                </div>
                <div className="field">
                  <label htmlFor="new">Nueva contraseña</label>
                  <Input
                    id="new"
                    name="new"
                    type="password"
                    autoComplete="new-password"
                    required
                    minLength={12}
                    maxLength={128}
                  />
                  <small className="muted">Entre 12 y 128 caracteres.</small>
                </div>
                <p className="muted">
                  Se cerrarán todas las sesiones de tu cuenta.
                </p>
                <Button disabled={busy} type="submit">
                  Actualizar contraseña
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      )}
    </>
  );
}
