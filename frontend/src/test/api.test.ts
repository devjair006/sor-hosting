import { test } from "node:test";
import assert from "node:assert/strict";
import {
  api,
  resetSession,
  ApiError,
  StaleSessionError,
} from "../services/api.ts";
test("401 invalidates session and exposes a typed error", async () => {
  const old = globalThis.fetch;
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({ error: { code: "UNAUTHENTICATED", message: "Caducó" } }),
      { status: 401 },
    );
  try {
    await assert.rejects(
      api("/sites"),
      (e: unknown) => e instanceof ApiError && e.status === 401,
    );
  } finally {
    globalThis.fetch = old;
    resetSession();
  }
});
test("a late private response is rejected after logout", async () => {
  const old = globalThis.fetch;
  let release!: (r: Response) => void;
  globalThis.fetch = () =>
    new Promise((resolve) => {
      release = resolve;
    });
  try {
    const pending = api("/sites");
    resetSession();
    release(new Response('{"items":[{"domain":"private.example"}]}'));
    await assert.rejects(pending, StaleSessionError);
  } finally {
    globalThis.fetch = old;
  }
});
test("mutation fetches CSRF and passes it without storing session tokens", async () => {
  const old = globalThis.fetch;
  const requests: RequestInit[] = [];
  globalThis.fetch = async (path, init) => {
    requests.push(init ?? {});
    return String(path).endsWith("/auth/csrf")
      ? new Response('{"csrf_token":"csrf-value"}')
      : new Response(null, { status: 204 });
  };
  try {
    resetSession();
    await api("/auth/logout", { method: "POST" });
    assert.equal(
      new Headers(requests[1]?.headers).get("X-CSRF-Token"),
      "csrf-value",
    );
    assert.equal(requests[1]?.credentials, "same-origin");
  } finally {
    globalThis.fetch = old;
    resetSession();
  }
});
test("transport failure is actionable in Spanish", async () => {
  const old = globalThis.fetch;
  globalThis.fetch = async () => {
    throw new TypeError("Failed to fetch");
  };
  try {
    await assert.rejects(
      api("/sites"),
      (e: unknown) =>
        e instanceof ApiError &&
        e.code === "NETWORK_ERROR" &&
        e.message.includes("conectar"),
    );
  } finally {
    globalThis.fetch = old;
    resetSession();
  }
});
