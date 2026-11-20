import { describe, expect, test } from "vitest";
import { useAppStore } from "../store/app";
import { json, mockFetch } from "../test/fetch";
import { ApiError, apiFetch, verifyToken } from "./client";

function login(token = "t") {
  useAppStore.setState({ token });
}

describe("apiFetch", () => {
  test("sends bearer token", async () => {
    login("t");
    const calls = mockFetch(() => json(200, []));
    await apiFetch("/api/alvos");
    expect(calls[0]?.url).toBe("/api/alvos");
    expect(calls[0]?.headers.get("Authorization")).toBe("Bearer t");
  });

  test("apiFetch clears token on 401", async () => {
    login();
    mockFetch(() => json(401, { erro: "não autorizado" }));
    const err = await apiFetch("/api/alvos").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
    expect(useAppStore.getState().token).toBeNull();
    expect(useAppStore.getState().sessionExpired).toBe(true);
  });

  test("maps server field error", async () => {
    login();
    mockFetch(() => json(400, { erro: "host inválido", campo: "host" }));
    const err = (await apiFetch("/api/alvos", { method: "POST", body: "{}" }).catch((e: unknown) => e)) as ApiError;
    expect(err.message).toBe("host inválido");
    expect(err.field).toBe("host");
  });

  test("handles 204 and non-JSON errors", async () => {
    login();
    mockFetch(() => new Response(null, { status: 204 }));
    await expect(apiFetch("/api/alvos/1", { method: "DELETE" })).resolves.toBeUndefined();
    mockFetch(() => new Response("oops", { status: 500 }));
    const err = (await apiFetch("/api/alvos").catch((e: unknown) => e)) as ApiError;
    expect(err.message).toBe("erro HTTP 500");
    expect(err.status).toBe(500);
  });

  test("sets content type only with body", async () => {
    login();
    const calls = mockFetch(() => json(200, {}));
    await apiFetch("/api/alvos", { method: "POST", body: "{}" });
    await apiFetch("/api/alvos");
    expect(calls[0]?.headers.get("Content-Type")).toBe("application/json");
    expect(calls[1]?.headers.get("Content-Type")).toBeNull();
  });
});

describe("verifyToken", () => {
  test("maps responses without touching the store", async () => {
    mockFetch(() => json(200, []));
    expect(await verifyToken("x")).toBe("ok");
    mockFetch(() => json(401, { erro: "não autorizado" }));
    expect(await verifyToken("x")).toBe("invalido");
    mockFetch(() => Promise.reject(new TypeError("rede")));
    expect(await verifyToken("x")).toBe("sem-conexao");
    expect(useAppStore.getState().token).toBeNull();
    expect(useAppStore.getState().sessionExpired).toBe(false);
  });
});
