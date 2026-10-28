export const sessionEvents = new EventTarget();
let generation = 0;
let csrf: Promise<string> | null = null;
export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}
export class StaleSessionError extends Error {
  constructor() {
    super("La sesión cambió durante la solicitud.");
  }
}
export function resetSession() {
  generation++;
  csrf = null;
  sessionEvents.dispatchEvent(new Event("changed"));
}
async function csrfToken(signal?: AbortSignal | null): Promise<string> {
  if (!csrf) {
    const expected = generation;
    csrf = transport("/api/v1/auth/csrf", {
      credentials: "same-origin",
      signal,
    })
      .then(async (res) => {
        if (!res.ok) {
          if (res.status === 401) resetSession();
          throw new ApiError(
            res.status,
            "CSRF_UNAVAILABLE",
            "Inicia sesión para continuar.",
          );
        }
        const v = (await res.json()) as { csrf_token: string };
        if (expected !== generation) throw new StaleSessionError();
        return v.csrf_token;
      })
      .catch((e) => {
        csrf = null;
        throw e;
      });
  }
  return csrf;
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const expected = generation;
  const method = options.method ?? "GET";
  const headers = new Headers(options.headers);
  if (options.body) headers.set("Content-Type", "application/json");
  if (!["GET", "HEAD"].includes(method) && path !== "/auth/login")
    headers.set("X-CSRF-Token", await csrfToken(options.signal));
  const res = await transport("/api/v1" + path, {
    ...options,
    headers,
    credentials: "same-origin",
  });
  if (expected !== generation) throw new StaleSessionError();
  if (!res.ok) {
    let error: { code: string; message: string } = {
      code: "REQUEST_FAILED",
      message: "No se pudo completar la solicitud.",
    };
    try {
      const value = (await res.json()) as { error?: typeof error };
      if (value.error) error = value.error;
    } catch {
      /* proxy or unavailable service */
    }
    if (res.status === 401 && path !== "/auth/me" && path !== "/auth/login")
      resetSession();
    throw new ApiError(res.status, error.code, error.message);
  }
  if (res.status === 204) return undefined as T;
  const data = (await res.json()) as T;
  if (expected !== generation) throw new StaleSessionError();
  return data;
}
export const key = () => crypto.randomUUID();
export function submit<T>(path: string, data: unknown, idempotencyKey: string) {
  return api<T>(path, {
    method: "POST",
    body: JSON.stringify(data),
    headers: { "Idempotency-Key": idempotencyKey },
  });
}

async function transport(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  try {
    return await fetch(input, init);
  } catch (e) {
    if (e instanceof Error && e.name === "AbortError") throw e;
    throw new ApiError(
      0,
      "NETWORK_ERROR",
      "No se pudo conectar con el panel. Comprueba la conexión e inténtalo de nuevo.",
    );
  }
}
export async function listAll<T>(
  path: string,
  signal?: AbortSignal,
): Promise<{ items: T[]; next_cursor: null }> {
  const items: T[] = [];
  let cursor = "";
  for (let i = 0; i < 100; i++) {
    const page = await api<{ items: T[]; next_cursor: string | null }>(
      path +
        (path.includes("?") ? "&" : "?") +
        "limit=100&cursor=" +
        encodeURIComponent(cursor),
      { signal },
    );
    items.push(...page.items);
    if (!page.next_cursor) return { items, next_cursor: null };
    cursor = page.next_cursor;
  }
  throw new ApiError(
    422,
    "LIST_LIMIT",
    "La lista es demasiado extensa. Reduce los resultados.",
  );
}
