import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type App } from "@/shared/api/client";

export type Upload = { pem?: string; pfx?: string; password?: string };

/** The site's own certificate and how visitors reach it. */
export function useCertificateActions(app: string) {
  const qc = useQueryClient();
  const store = (a: App) => {
    qc.setQueryData(["app", app], a);
    void qc.invalidateQueries({ queryKey: ["apps"] });
  };
  const upload = useMutation({
    mutationFn: (body: Upload) => unwrap(api.PUT("/apps/{app}/certificate", { params: { path: { app } }, body })),
    onSuccess: store,
  });
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/apps/{app}/certificate", { params: { path: { app } } })),
    onSuccess: store,
  });
  const setScheme = useMutation({
    mutationFn: (public_scheme: "https" | "http") =>
      unwrap(api.PATCH("/apps/{app}", { params: { path: { app } }, body: { public_scheme } })),
    onSuccess: store,
  });
  return { upload, remove, setScheme };
}

/** Reads a chosen file: a .pfx as base64, PEM files as text. */
export async function readCertificateFile(file: File): Promise<Upload> {
  if (/\.(pfx|p12)$/i.test(file.name)) {
    const bytes = new Uint8Array(await file.arrayBuffer());
    let bin = "";
    for (const b of bytes) bin += String.fromCharCode(b);
    return { pfx: btoa(bin) };
  }
  return { pem: await file.text() };
}
