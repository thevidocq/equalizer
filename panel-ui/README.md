# panel-ui

Web panel in TypeScript + React (Vite, React Router, TanStack Query). Custom components in `src/shared/ui`, built on Radix UI and Tailwind CSS.

```text
src/
  app/        bootstrap, router, providers
  shared/     ui/, api/ (client generated from OpenAPI), hooks/
  features/   applications/ site-overview/ status/ logs/ siem-forwarding/
              import-export/ connectors/ proposals/ map-editor/ preview/
              crawls/ llm-settings/ events/
```

Rules: a feature exposes only its `index.ts`; pages compose components; data comes only from hooks; 200-line limit per file. Status: not implemented yet.
