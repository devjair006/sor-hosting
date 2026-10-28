import { Suspense, lazy } from "react";
import {
  Navigate,
  Outlet,
  RouterProvider,
  createBrowserRouter,
} from "react-router-dom";
import { useSession } from "./hooks/useSession";
import { AppShell } from "./components/layout/AppShell";
import { Loading } from "./components/State";
const LoginPage = lazy(() =>
  import("./pages/LoginPage").then((m) => ({ default: m.LoginPage })),
);
const DashboardPage = lazy(() =>
  import("./pages/DashboardPage").then((m) => ({ default: m.DashboardPage })),
);
const SitesPage = lazy(() =>
  import("./pages/SitesPage").then((m) => ({ default: m.SitesPage })),
);
const SiteDetailPage = lazy(() =>
  import("./pages/SiteDetailPage").then((m) => ({ default: m.SiteDetailPage })),
);
const ContainersPage = lazy(() =>
  import("./pages/ContainersPage").then((m) => ({ default: m.ContainersPage })),
);
const DomainsPage = lazy(() =>
  import("./pages/DomainsPage").then((m) => ({ default: m.DomainsPage })),
);
const BackupsPage = lazy(() =>
  import("./pages/BackupsPage").then((m) => ({ default: m.BackupsPage })),
);
const CreateSitePage = lazy(() =>
  import("./pages/CreateSitePage").then((m) => ({ default: m.CreateSitePage })),
);
const AuditPage = lazy(() =>
  import("./pages/AuditPage").then((m) => ({ default: m.AuditPage })),
);
const SettingsPage = lazy(() =>
  import("./pages/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);
function Protected() {
  const s = useSession();
  if (s.loading) return <Loading />;
  return s.user ? <Outlet /> : <Navigate to="/login" replace />;
}
const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  {
    element: <Protected />,
    children: [
      {
        element: <AppShell />,
        children: [
          { path: "/", element: <DashboardPage /> },
          { path: "/sites", element: <SitesPage /> },
          { path: "/sites/new", element: <CreateSitePage /> },
          { path: "/sites/:id", element: <SiteDetailPage /> },
          { path: "/containers", element: <ContainersPage /> },
          { path: "/domains", element: <DomainsPage /> },
          { path: "/backups", element: <BackupsPage /> },
          { path: "/audit", element: <AuditPage /> },
          { path: "/settings", element: <SettingsPage /> },
          {
            path: "*",
            element: (
              <section>
                <h1>Página no encontrada</h1>
                <a href="/">Volver al resumen</a>
              </section>
            ),
          },
        ],
      },
    ],
  },
]);
export function App() {
  return (
    <Suspense fallback={<Loading />}>
      <RouterProvider router={router} />
    </Suspense>
  );
}
