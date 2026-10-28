import { useState, useRef, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Check, ArrowLeft, ArrowRight, ShieldCheck } from "lucide-react";
import { PageHeader } from "../components/layout/AppShell";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { useOperation } from "../hooks/useOperation";
import { useJob } from "../hooks/useJob";
import { OperationFeedback } from "../components/OperationFeedback";
export function CreateSitePage() {
  const [step, setStep] = useState(0);
  const [form, setForm] = useState({
    name: "",
    domain: "",
    wordpress_title: "",
    admin_email: "",
    admin_password: "",
    memory_mb: 512,
    cpu_limit: 1,
  });
  const formRef = useRef<HTMLFormElement>(null);
  const op = useOperation();
  const job = useJob(op.jobId);
  const steps = ["Dominio", "WordPress", "Recursos", "Revisión"];
  const field = (
    name:
      "name" | "domain" | "wordpress_title" | "admin_email" | "admin_password",
    label: string,
    type = "text",
  ) => (
    <div className="field">
      <label htmlFor={name}>{label}</label>
      <Input
        id={name}
        type={type}
        value={form[name]}
        onChange={(e) => setForm({ ...form, [name]: e.target.value })}
        required
        maxLength={name === "admin_password" ? 128 : 253}
        minLength={name === "admin_password" ? 12 : 1}
        autoComplete={
          name === "admin_password"
            ? "new-password"
            : name === "admin_email"
              ? "email"
              : "off"
        }
      />
      {name === "admin_password" && (
        <small className="muted">
          Entre 12 y 128 caracteres. Puedes pegar una contraseña de tu gestor.
        </small>
      )}
    </div>
  );
  async function finish(e: FormEvent) {
    e.preventDefault();
    if (step < 3) {
      setStep(step + 1);
      return;
    }
    const j = await op.run("/sites", {
      name: form.name,
      domain: form.domain,
      wordpress_title: form.wordpress_title,
      admin_email: form.admin_email,
      admin_password: form.admin_password,
      resources: { memory_mb: form.memory_mb, cpu_limit: form.cpu_limit },
    });
    if (j) setForm((v) => ({ ...v, admin_password: "" }));
  }
  return (
    <>
      <PageHeader
        title="Crear sitio WordPress"
        description="Prepara tu dominio, configura WordPress y revisa los recursos."
        action={false}
      />
      <ol className="wizard-steps">
        {steps.map((s, i) => (
          <li
            key={s}
            aria-current={i === step ? "step" : undefined}
            className={i <= step ? "current" : ""}
          >
            <span>{i < step ? <Check size={16} /> : i + 1}</span>
            {s}
          </li>
        ))}
      </ol>
      <Card className="wizard-card">
        <CardHeader>
          <CardTitle>
            {op.jobId ? "Progreso de la instalación" : steps[step]}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {!op.jobId ? (
            <form ref={formRef} onSubmit={finish}>
              {step === 0 ? (
                <>
                  {field("name", "Nombre del sitio")}
                  {field("domain", "Dominio completo")}
                  <p className="info-note">
                    Apunta el registro A/AAAA de este dominio a la IP pública de
                    tu VPS. Si el DNS aún no está listo, podrás completar SSL
                    después.
                  </p>
                </>
              ) : step === 1 ? (
                <>
                  {field("wordpress_title", "Título de WordPress")}
                  {field("admin_email", "Correo del administrador", "email")}
                  {field(
                    "admin_password",
                    "Contraseña de WordPress",
                    "password",
                  )}
                </>
              ) : step === 2 ? (
                <>
                  <div className="field">
                    <label htmlFor="memory">Memoria para WordPress (MB)</label>
                    <Input
                      id="memory"
                      type="number"
                      min={256}
                      max={16384}
                      step={256}
                      value={form.memory_mb}
                      onChange={(e) =>
                        setForm({ ...form, memory_mb: Number(e.target.value) })
                      }
                      required
                    />
                  </div>
                  <div className="field">
                    <label htmlFor="cpu">Límite de CPU</label>
                    <Input
                      id="cpu"
                      type="number"
                      min={0.25}
                      max={16}
                      step={0.25}
                      value={form.cpu_limit}
                      onChange={(e) =>
                        setForm({ ...form, cpu_limit: Number(e.target.value) })
                      }
                      required
                    />
                  </div>
                  <p className="muted">
                    El sitio tendrá una base de datos MariaDB y volúmenes
                    independientes.
                  </p>
                </>
              ) : (
                <>
                  <dl className="definition-list">
                    <dt>Sitio</dt>
                    <dd>{form.name}</dd>
                    <dt>Dominio</dt>
                    <dd>{form.domain}</dd>
                    <dt>WordPress</dt>
                    <dd>{form.wordpress_title}</dd>
                    <dt>Administrador</dt>
                    <dd>{form.admin_email}</dd>
                    <dt>Recursos web</dt>
                    <dd>
                      {form.memory_mb} MB · {form.cpu_limit} CPU
                    </dd>
                  </dl>
                  <p className="info-note">
                    <ShieldCheck size={20} />
                    Se crearán contenedores, red, volúmenes y credenciales
                    propias para este sitio.
                  </p>
                </>
              )}
              {op.error && (
                <p role="alert" className="field-error">
                  {op.error}
                </p>
              )}
              <div className="wizard-actions">
                <Button
                  type="button"
                  variant="outline"
                  disabled={step === 0 || op.busy}
                  onClick={() => setStep(step - 1)}
                >
                  <ArrowLeft size={16} />
                  Volver
                </Button>
                <Button type="submit" disabled={op.busy}>
                  {step === 3 ? "Crear sitio" : "Continuar"}
                  <ArrowRight size={16} />
                </Button>
              </div>
            </form>
          ) : (
            <>
              <OperationFeedback {...op} />
              {job.data?.job.state === "succeeded" && (
                <Button asChild>
                  <Link to="/sites">Ver mis sitios</Link>
                </Button>
              )}
              <p className="muted small">
                Puedes salir de esta pantalla. El trabajo continuará en el
                servidor y quedará registrado en Auditoría.
              </p>
            </>
          )}
        </CardContent>
      </Card>
    </>
  );
}
