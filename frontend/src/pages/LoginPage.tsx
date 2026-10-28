import { useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { Check, Eye, EyeOff, LoaderCircle } from "lucide-react";
import { Brand } from "../components/Brand";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { useSession } from "../hooks/useSession";
export function LoginPage() {
  const { user, loading, login } = useSession();
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  if (user) return <Navigate to="/" replace />;
  async function handle(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await login(email, password);
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo iniciar sesión.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="login">
      <section className="login-brand">
        <Brand />
        <div>
          <p className="login-headline">
            EL CONTROL,
            <br />
            BIEN ORGANIZADO.
          </p>
          <p className="login-subtitle">
            Tu infraestructura merece claridad.
            <br />
            Administra tus sitios con SOR Hosting.
          </p>
          <ul className="brand-benefits">
            {[
              [
                "Una vista de todo",
                "Recursos, servicios y sitios en contexto.",
              ],
              [
                "Operaciones trazables",
                "Cada cambio tiene un estado y un registro.",
              ],
              [
                "Tu servidor, tu panel",
                "Una herramienta creada para tu infraestructura.",
              ],
            ].map(([title, desc]) => (
              <li key={title}>
                <span>
                  <Check aria-hidden="true" />
                </span>
                <div>
                  <strong>{title}</strong>
                  <p>{desc}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <small>SOR HOSTING / ADMINISTRACIÓN DE INFRAESTRUCTURA</small>
      </section>
      <section className="login-form-panel">
        <form className="login-form" onSubmit={handle}>
          <p className="eyebrow">BIENVENIDO A SOR</p>
          <h1>Accede a tu panel</h1>
          <p className="muted">Inicia sesión con tu cuenta de administrador.</p>
          <div className="field">
            <label htmlFor="email">Correo electrónico</label>
            <Input
              id="email"
              name="email"
              type="email"
              autoComplete="username"
              required
              maxLength={254}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="administrador@tu-dominio.com"
            />
          </div>
          <div className="field">
            <label htmlFor="password">Contraseña</label>
            <div className="password-field">
              <Input
                id="password"
                name="password"
                type={show ? "text" : "password"}
                autoComplete="current-password"
                required
                maxLength={128}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-describedby={error ? "login-error" : undefined}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={show ? "Ocultar contraseña" : "Mostrar contraseña"}
                aria-pressed={show}
                onClick={() => setShow(!show)}
              >
                {show ? <EyeOff size={19} /> : <Eye size={19} />}
              </Button>
            </div>
          </div>
          {error && (
            <p id="login-error" role="alert" className="field-error">
              {error}
            </p>
          )}
          <Button
            disabled={busy || loading}
            className="login-submit"
            type="submit"
          >
            {busy ? (
              <LoaderCircle size={18} className="spin" aria-hidden="true" />
            ) : null}
            Iniciar sesión <span aria-hidden="true">→</span>
          </Button>
          <p className="login-security">
            Acceso exclusivo para administradores.
            <br />
            Tu sesión caduca automáticamente por seguridad.
          </p>
          <div className="login-help">
            ¿Necesitas recuperar el acceso?
            <br />
            <strong>Contacta al administrador del servidor.</strong>
          </div>
        </form>
      </section>
    </main>
  );
}
