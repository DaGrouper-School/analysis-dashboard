# DaGrouper Analysis Dashboard

Monorepo for the DaGrouper Analysis Platform.

| Folder                  | What it is                                                              |
| ----------------------- | ----------------------------------------------------------------------- |
| [`frontend/`](frontend) | SvelteKit app. Runs on mock data saved in localStorage for now.         |
| [`backend/`](backend)   | Placeholder for the auth API and SAM integration. Stack not chosen yet. |

The folders are npm workspaces. From the repo root:

```sh
npm install
npm run dev      # frontend dev server
npm test         # tests in every workspace
npm run build    # frontend static build
```

Pushes to `main` deploy the front end to GitHub Pages (`.github/workflows/deploy-pages.yml`). In
the repo's **Settings → Pages**, the source must be set to **GitHub Actions**.
