import { useState } from "react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Globe,
  Container,
  ShieldCheck,
  Archive,
  ScrollText,
  Settings,
  LogOut,
  Menu,
  Plus,
} from "lucide-react";
import { Brand } from "../Brand";
import { Button } from "../ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "../ui/dialog";
import { useSession } from "../../hooks/useSession";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../services/api";
const links = [
  { to: "/", label: "Resumen", icon: LayoutDashboard },
  { to: "/sites", label: "Sitios WordPress", icon: Globe },
  { to: "/containers", label: "Contenedores", icon: Container },
  { to: "/domains", label: "Dominios y SSL", icon: ShieldCheck },
  { to: "/backups", label: "Backups", icon: Archive },
];
export function AppShell() {
  const [open, setOpen] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  const { user, logout } = useSession();
  const location = useLocation();
  const health = useQuery({
    queryKey: ["settings"],
    queryFn: ({ signal }) =>
      api<{ server: { name: string }; integrations: { agent: boolean } }>(
        "/settings",
        { signal },
      ),
    refetchInterval: 30000,
  });
  const nav = (
    <>
      <Brand />
      <p className="nav-label">ESPACIO DE TRABAJO</p>
      <nav aria-label="Navegación principal">
        {links.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            onClick={() => setOpen(false)}
          >
            <Icon size={19} aria-hidden="true" />
            {label}
          </NavLink>
        ))}
      </nav>
      <p className="nav-label">ADMINISTRACIÓN</p>
      <nav aria-label="Administración">
        {[
          { to: "/audit", label: "Auditoría", icon: ScrollText },
          { to: "/settings", label: "Configuración", icon: Settings },
        ].map(({ to, label, icon: Icon }) => (
          <NavLink key={to} to={to} onClick={() => setOpen(false)}>
            <Icon size={18} aria-hidden="true" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="sidebar-bottom">
        <div className="agent-card">
          <span
            className={
              "status-dot " +
              (!health.data?.integrations.agent ? "dot-muted" : "")
            }
          />
          <strong>
            {health.data?.integrations.agent
              ? "Agente conectado"
              : "Agente sin conexión"}
          </strong>
          <p>{health.data?.server.name ?? "VPS principal"}</p>
          <small>Conexión local protegida</small>
        </div>
        <div className="account">
          <span className="avatar">AD</span>
          <div>
            <strong>Administrador</strong>
            <small>{user?.email}</small>
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Cerrar sesión"
            onClick={() =>
              void logout().catch((e) =>
                setLogoutError(
                  e instanceof Error ? e.message : "No se pudo cerrar sesión",
                ),
              )
            }
          >
            <LogOut size={17} />
          </Button>
        </div>
        {logoutError && <p role="alert">{logoutError}</p>}
      </div>
    </>
  );
  const title =
    location.pathname === "/sites/new"
      ? "Crear sitio"
      : (links.find((x) =>
          x.to === "/"
            ? location.pathname === "/"
            : location.pathname.startsWith(x.to),
        )?.label ??
        (location.pathname === "/audit" ? "Auditoría" : "Configuración"));
  return (
    <>
      <a className="skip-link" href="#content">
        Saltar al contenido
      </a>
      <aside className="sidebar">{nav}</aside>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="mobile-nav">
          <DialogTitle className="sr-only">
            Navegación de SOR Hosting
          </DialogTitle>
          <DialogDescription className="sr-only">
            Selecciona un módulo del panel.
          </DialogDescription>
          {nav}
        </DialogContent>
      </Dialog>
      <div className="app-main">
        <header className="topbar">
          <Button
            variant="ghost"
            size="icon"
            className="mobile-menu"
            aria-label="Abrir navegación"
            onClick={() => setOpen(true)}
          >
            <Menu />
          </Button>
          <div className="breadcrumb">
            <span>Infraestructura</span>
            <span aria-hidden="true">/</span>
            <strong>{title}</strong>
          </div>
          <div className="server-pill">
            <span
              className={
                "status-dot " +
                (!health.data?.integrations.agent ? "dot-muted" : "")
              }
            />
            Azure VPS
          </div>
        </header>
        <main id="content" className="page-content">
          <Outlet />
        </main>
        <footer className="app-footer">
          <span>SOR Hosting</span>
          <span>Administración de infraestructura</span>
        </footer>
      </div>
    </>
  );
}
export function PageHeader({
  eyebrow = "SOR HOSTING",
  title,
  description,
  action = true,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  action?: boolean;
}) {
  return (
    <div className="page-header">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p className="muted">{description}</p>
      </div>
      {action && (
        <Button asChild>
          <Link to="/sites/new">
            <Plus size={18} aria-hidden="true" />
            Crear sitio
          </Link>
        </Button>
      )}
    </div>
  );
}
