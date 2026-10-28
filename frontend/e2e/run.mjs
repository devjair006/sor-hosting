import { chromium } from "playwright";
import assert from "node:assert/strict";
import { readFile, mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const id = "a".repeat(48),
  cid = "b".repeat(64),
  protectedID = "c".repeat(64),
  now = new Date().toISOString();
const site = {
  id,
  name: "Sitio de prueba",
  domain: "sitio.example",
  state: "active",
  availability: true,
  created_at: now,
  updated_at: now,
};
const rows = [
  {
    id: cid,
    name: "wordpress-web",
    image: "wordpress@sha256:test",
    state: "running",
    status: "Up",
    managed: true,
    can_manage: true,
    can_read_logs: true,
    protected: false,
    site_id: id,
    cpu_pct: 10,
    memory_bytes: 120000000,
  },
  {
    id: protectedID,
    name: "prometheus-protegido",
    image: "prom/prometheus",
    state: "running",
    status: "Up",
    managed: false,
    can_manage: false,
    can_read_logs: false,
    protected: true,
    site_id: "",
    cpu_pct: null,
    memory_bytes: null,
  },
];
let authenticated = false;
const errors = [];
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.BROWSER_EXECUTABLE || undefined,
  args: ["--no-sandbox"],
});
try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1080 },
    reducedMotion: "reduce",
  });
  page.on("pageerror", (e) => errors.push(e.message));
  await page.route("**/*", async (route) => {
    const request = route.request(),
      url = new URL(request.url()),
      p = url.pathname;
    const json = (body, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(body),
      });
    if (p.startsWith("/api/v1/")) {
      const endpoint = p.slice(7);
      if (endpoint === "/auth/login") {
        const body = request.postDataJSON();
        if (body.password !== "test-password-123")
          return json(
            {
              error: {
                code: "INVALID_CREDENTIALS",
                message: "Correo o contraseña incorrectos.",
              },
            },
            401,
          );
        authenticated = true;
        return json({
          user: { id, email: "admin@example.com", role: "admin" },
        });
      }
      if (!authenticated)
        return json(
          { error: { code: "UNAUTHENTICATED", message: "Inicia sesión." } },
          401,
        );
      if (endpoint === "/auth/me")
        return json({
          user: { id, email: "admin@example.com", role: "admin" },
        });
      if (endpoint === "/auth/csrf") return json({ csrf_token: "test-csrf" });
      if (!["GET", "HEAD"].includes(request.method()))
        assert.equal(request.headers()["x-csrf-token"], "test-csrf");
      if (endpoint === "/auth/logout") {
        authenticated = false;
        return route.fulfill({ status: 204 });
      }
      if (endpoint === "/settings")
        return json({
          server: { id: "local", name: "VPS de prueba" },
          integrations: { agent: true, prometheus: true },
          backup_destination: "Directorio local del agente",
        });
      if (endpoint === "/dashboard")
        return json({
          server: { id: "local", name: "VPS de prueba" },
          metrics: {
            cpu: 24.8,
            memory_pct: 40,
            memory_used: 3200000000,
            memory_total: 8000000000,
            disk_used: 42000000000,
            disk_total: 128000000000,
            traffic: 1240000,
          },
          services: ["Docker Engine", "Nginx", "Prometheus", "Agente SOR"].map(
            (name) => ({ name, available: true }),
          ),
          sites: [site],
          recent_activity: [],
          observed_at: now,
        });
      if (endpoint === "/monitoring")
        return json({
          series: ["cpu", "memory"].map((name, j) => ({
            name,
            unit: "%",
            points: Array.from({ length: 20 }, (_, i) => ({
              timestamp: Math.floor(Date.now() / 1000) - (20 - i) * 60,
              value: j ? 40 : 20 + (i % 5),
            })),
          })),
          observed_at: now,
          stale: false,
        });
      if (endpoint === "/sites" && request.method() === "GET")
        return json({ items: [site], next_cursor: null });
      if (endpoint === "/sites/" + id)
        return json({ site, containers: [rows[0]], domains: [], backups: [] });
      if (endpoint === "/containers")
        return json({ items: rows, next_cursor: null });
      if (
        endpoint === "/domains" ||
        endpoint === "/certificates" ||
        endpoint === "/backups" ||
        endpoint === "/audit"
      )
        return json({ items: [], next_cursor: null });
      if (endpoint.endsWith("/logs"))
        return json({
          lines: ["Salida del contenedor de prueba"],
          truncated: false,
        });
      if (endpoint.startsWith("/jobs/"))
        return json({
          job: {
            id,
            kind: "container.restart",
            resource_id: cid,
            state: "succeeded",
            phase: "Operación completada",
            error_code: "",
            created_at: now,
            updated_at: now,
          },
        });
      if (request.method() === "POST")
        return json(
          {
            job: {
              id,
              kind: "container.restart",
              resource_id: cid,
              state: "queued",
              phase: "Pendiente",
              error_code: "",
              created_at: now,
              updated_at: now,
            },
          },
          202,
        );
      return json(
        { error: { code: "NOT_FOUND", message: "Fixture no encontrado" } },
        404,
      );
    }
    const relative = p.startsWith("/assets/") ? p.slice(1) : "index.html";
    const mime = relative.endsWith(".js")
      ? "application/javascript"
      : relative.endsWith(".css")
        ? "text/css"
        : relative.endsWith(".ttf")
          ? "font/ttf"
          : "text/html";
    return route.fulfill({
      status: 200,
      contentType: mime,
      body: await readFile(path.join(root, "dist", relative)),
    });
  });
  await page.goto("https://sor.test/login");
  await page.getByLabel("Correo electrónico").fill("admin@example.com");
  await page.getByLabel("Contraseña", { exact: true }).fill("incorrecta");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.getByRole("alert").filter({ hasText: "incorrectos" }).waitFor();
  assert.equal(
    await page.getByLabel("Correo electrónico").inputValue(),
    "admin@example.com",
  );
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("test-password-123");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.getByRole("heading", { name: "Resumen del servidor" }).waitFor();
  await mkdir(path.join(root, "test-results"), { recursive: true });
  await page.screenshot({
    path: path.join(root, "test-results/dashboard.png"),
    fullPage: true,
  });
  await page
    .getByRole("navigation", { name: "Navegación principal" })
    .getByRole("link", { name: "Contenedores", exact: true })
    .click();
  await page.getByText("prometheus-protegido", { exact: true }).waitFor();
  assert.equal(
    await page
      .getByRole("button", { name: "Detener prometheus-protegido" })
      .count(),
    0,
  );
  await page.getByRole("button", { name: "Reiniciar wordpress-web" }).click();
  await page.getByRole("dialog").waitFor();
  await page.getByRole("button", { name: "Confirmar", exact: true }).click();
  await page.getByText("Operación completada", { exact: true }).waitFor();
  for (const [route, title] of [
    ["sites", "Sitios WordPress"],
    ["domains", "Dominios y SSL"],
    ["backups", "Backups"],
    ["audit", "Auditoría"],
    ["settings", "Configuración"],
    ["sites/new", "Crear sitio WordPress"],
  ]) {
    await page.goto("https://sor.test/" + route);
    await page.getByRole("heading", { name: title, exact: true }).waitFor();
  }
  for (const width of [375, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("https://sor.test/");
    await page.getByRole("heading", { name: "Resumen del servidor" }).waitFor();
    assert.ok(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth + 1,
      ),
      "El documento desborda a " + width,
    );
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page
    .getByRole("button", { name: "Cerrar sesión", exact: true })
    .click();
  await page.getByRole("heading", { name: "Accede a tu panel" }).waitFor();
  assert.deepEqual(errors, [], "Errores JavaScript de navegador");
  console.log(
    "E2E: login/error, navegación, recurso protegido, job, logout y 4 tamaños verificados con API controlada.",
  );
} finally {
  await browser.close();
}
