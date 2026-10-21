# intellisim.front

Web UI for IntelliSim — an LLM-driven meta-system for agent-based modeling. Provides chat-based
model creation, a visual model editor with a PixiJS environment canvas, a live simulation runner
(WebSocket step stream), a configurable widget dashboard, and an LLM results-analysis view.

Built with Vue 3 (`<script setup>` + TypeScript), PrimeVue, Tailwind CSS, PixiJS, Chart.js,
Pinia and Vite.

## Commands

```sh
npm install
npm run dev          # dev server on http://localhost:3000 (proxies /api and /ws to localhost:8080)
npm run build        # type-check (vue-tsc) + production build
npx playwright test  # e2e suites (require the full stack running)
```

## Configuration

Development defaults live in `.env.development` (API base URL, WebSocket URL, OIDC authority
`http://localhost:9090`, client id `intellisim-front`). The auth variables must match the core
backend's `AUTH_SERVER`.

Launching the full IntelliSim stack (MongoDB, core backend, LLM provider, auth mock) is described
in the repository root `README.md`; deeper frontend documentation lives in `docs/`.

## License

Copyright (c) 2026 Vladyslav Pryshchepa.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE) — free to use, modify and
share for **noncommercial purposes** such as scientific research, academic study and teaching.

**Commercial use requires a separate license.** Contact vlad.prishhepa@gmail.com.
