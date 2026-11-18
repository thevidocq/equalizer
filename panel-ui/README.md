# panel-ui

Pannello web in TypeScript + React (Vite, React Router, TanStack Query). Componenti propri in `src/shared/ui` su Radix UI e Tailwind CSS.

```text
src/
  app/        avvio, router, provider
  shared/     ui/, api/ (client generato da OpenAPI), hooks/
  features/   applications/ site-overview/ status/ logs/ siem-forwarding/
              import-export/ connectors/ proposals/ map-editor/ preview/
              crawls/ llm-settings/ events/
```

Regole: una funzionalità espone solo il suo `index.ts`; pagine che compongono componenti; dati solo da hook; soglia 200 righe per file. Stato: da implementare.
