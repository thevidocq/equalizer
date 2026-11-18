import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type User = Schemas["User"];
export type Tenant = Schemas["Tenant"];
export type App = Schemas["App"];
export type Connector = Schemas["Connector"];
export type ConnectorCreated = Schemas["ConnectorCreated"];
export type MapVersion = Schemas["MapVersion"];
export type MapStatus = MapVersion["status"];
export type AuditEntry = Schemas["AuditEntry"];

/** Error raised for every non-2xx answer, with the contract's error body. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly details: string[] = [],
  ) {
    super(message);
  }
}

// The CSRF barrier of panel-api: every request carries this header.
const csrf: Middleware = {
  onRequest({ request }) {
    request.headers.set("X-Eq-Panel", "1");
    return request;
  },
};

export const api = createClient<paths>({ baseUrl: "/v1", credentials: "same-origin" });
api.use(csrf);

type Result<T> = { data?: T; error?: unknown; response: Response };

/** Returns the data or throws an ApiError, so hooks only see success. */
export async function unwrap<T>(p: Promise<Result<T>>): Promise<T> {
  const { data, error, response } = await p;
  if (response.ok) return data as T;
  const body = (error ?? {}) as Partial<Schemas["Error"]>;
  throw new ApiError(response.status, body.error ?? "unknown", body.message ?? response.statusText, body.details ?? []);
}
