import { chromium } from "playwright";
import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, writeFile, mkdir, rm } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Real Go server and SQLite; no HTTP routes or authentication are mocked.
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const data = await mkdtemp(path.join(tmpdir(), "sor-live-"));
const listener = createServer();
listener.listen(0, "127.0.0.1");
await once(listener, "listening");
const port = listener.address().port;
await new Promise((resolve) => listener.close(resolve));
const origin = `http://127.0.0.1:${port}`;
const password = randomBytes(24).toString("hex");
const passwordFile = path.join(data, "password");
const email = "admin@example.invalid";
const env = { ...process.env, SOR_MODE: "development", SOR_ORIGIN: origin,
  SOR_ADDRESS: `127.0.0.1:${port}`, SOR_DATABASE: path.join(data, "panel.db"),
  SOR_STATIC_DIR: path.join(root, "frontend/dist"), SOR_SECRET_KEY: randomBytes(32).toString("base64"),
  SOR_AGENT_SOCKET: path.join(data, "absent-agent.sock"), SOR_PROMETHEUS_URL: "", SOR_PROMETHEUS_INSTANCE: "" };
const binary = process.env.SOR_SERVER_BINARY || path.join(root, "bin/sor-server");
let server, browser;
let diagnostics = "";
function run(args) {
  const child = spawn(binary, args, { env, stdio: ["ignore", "pipe", "pipe"] });
  child.stdout.on("data", (chunk) => { diagnostics += chunk; });
  child.stderr.on("data", (chunk) => { diagnostics += chunk; });
  return child;
}
async function start() {
  server = run(["serve"]);
  for (let i = 0; i < 100; i++) {
    if (server.exitCode !== null) throw new Error("Go server exited: " + diagnostics);
    try {
      if ((await fetch(origin + "/readyz")).ok) return;
    } catch { /* wait for listen */ }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("Go server readiness timeout: " + diagnostics);
}
async function stop() {
  if (server && server.exitCode === null) {
    const exited = once(server, "exit");
    server.kill("SIGTERM");
    await exited;
  }
}
try {
  await writeFile(passwordFile, password, { mode: 0o600 });
  assert.equal((await once(run(["create-admin", "--email", email, "--password-file", passwordFile]), "exit"))[0], 0, diagnostics);
  await start();
  browser = await chromium.launch({ headless: true, executablePath: process.env.BROWSER_EXECUTABLE || undefined, args: ["--no-sandbox"] });
  const context = await browser.newContext({ viewport: { width: 1440, height: 1080 }, reducedMotion: "reduce" });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  assert.equal((await context.request.get(origin + "/api/v1/sites")).status(), 401);
  assert.equal((await context.request.get(origin + "/api/v1/missing")).status(), 404);
  await page.goto(origin + "/settings");
  await page.getByLabel("Correo electrónico").fill(email);
  await page.getByLabel("Contraseña", { exact: true }).fill("incorrect-password");
  await page.getByRole("button", { name: /Iniciar sesión/ }).click();
  await page.getByRole("alert").waitFor();
  await page.getByLabel("Contraseña", { exact: true }).fill(password);
  await page.getByRole("button", { name: /Iniciar sesión/ }).click();
  await page.getByRole("heading", { name: "Resumen del servidor" }).waitFor();
  await page.getByText("Agente sin conexión", { exact: true }).waitFor();
  const cookie = (await context.cookies()).find((c) => c.name === "sor_dev_session");
  assert.ok(cookie?.httpOnly && cookie.sameSite === "Strict", "private session cookie flags");
  const dashboard = await context.request.get(origin + "/api/v1/dashboard");
  assert.equal(dashboard.status(), 200);
  assert.deepEqual((await dashboard.json()).sites, []);
  assert.equal(dashboard.headers()["cache-control"], "no-store");
  await page.getByText("Cargando información…", { exact: true }).waitFor({ state: "hidden" });
  await mkdir(path.join(root, "frontend/test-results"), { recursive: true });
  await page.screenshot({ path: path.join(root, "frontend/test-results/live-dashboard.png"), fullPage: true });
  await page.getByRole("link", { name: "Configuración", exact: true }).click();
  await page.getByLabel("Nombre del servidor").fill("VPS de integración");
  await page.getByRole("button", { name: "Guardar nombre" }).click();
  await page.getByRole("status").filter({ hasText: "Cambios guardados." }).waitFor();
  const denied = await context.request.patch(origin + "/api/v1/settings", { data: { server_name: "forbidden" } });
  assert.equal(denied.status(), 403, "writes without origin/CSRF must fail");
  const before = (await context.cookies()).find((c) => c.name === "sor_dev_session")?.value;
  await stop();
  await start();
  await page.reload();
  await page.getByLabel("Nombre del servidor").waitFor();
  assert.equal(await page.getByLabel("Nombre del servidor").inputValue(), "VPS de integración");
  assert.equal((await context.request.get(origin + "/api/v1/auth/me")).status(), 200, "session persists across API restart");
  await page.getByRole("link", { name: "Auditoría", exact: true }).click();
  await page.getByText("settings.rename", { exact: true }).waitFor();
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.getByRole("heading", { name: "Accede a tu panel" }).waitFor();
  assert.equal((await context.request.get(origin + "/api/v1/auth/me", { headers: { cookie: `sor_dev_session=${before}` } })).status(), 401, "logout revokes server-side session");
  assert.deepEqual(errors, []);
  console.log("Live Go + SQLite + Chromium: login, private cookies, dashboard, CSRF, settings, audit, persistence after restart and logout passed.");
} finally {
  await browser?.close();
  await stop();
  await rm(data, { recursive: true, force: true });
}
