# Setup & Environment Guide

This guide covers environment configuration, development server setup, build process, and deployment preparation for the IntelliSim frontend.

## Prerequisites

- **Node.js**: v18.0.0 or higher (check with `node --version`)
- **npm**: v8.0.0 or higher (included with Node.js)
- **Git**: For version control
- **Backend API**: IntelliSim backend running on http://localhost:8080

### System Requirements

- **OS**: Linux, macOS, or Windows
- **Disk Space**: 500 MB (node_modules + build artifacts)
- **RAM**: 2 GB minimum (4 GB recommended for development)

## Installation

### 1. Clone Repository

```bash
git clone <this-repository-url>
cd intellisim-experiment-artifacts/intellisim.front
```

### 2. Install Dependencies

```bash
npm install
```

**Duration**: 2-5 minutes depending on connection and disk speed.

**What Gets Installed**:
- Vue 3 and ecosystem (router, pinia)
- Build tools (Vite, TypeScript)
- UI libraries (PrimeVue, Tailwind CSS)
- Utilities (Axios, Chart.js, PixiJS)
- Dev tools (Playwright, vue-tsc)

**Output**: Creates `node_modules/` directory (~500 MB).

### 3. Verify Installation

```bash
npm run build
```

Should complete without errors (creates `dist/` folder).

---

## Environment Configuration

### Environment Files

The project uses three environment files:

| File | Purpose | Tracked |
|------|---------|---------|
| `.env` | Base defaults | ❌ NO (.gitignore) |
| `.env.development` | Dev overrides | ✅ YES (checked in) |
| `.env.production` | Production values | ❌ NO (checked in) |

### .env.development

**File**: `.env.development`

**Current Content**:
```
VITE_API_BASE_URL=/api/v1/intellisim
VITE_WS_BASE_URL=ws://localhost:8080/api/v1/intellisim
```

### Environment Variables

#### VITE_API_BASE_URL

- **Purpose**: Base URL for HTTP REST API calls
- **Type**: string (URL)
- **Default**: `/api/v1/intellisim`
- **Examples**:
  - Dev: `/api/v1/intellisim` (via proxy)
  - Staging: `https://staging.intellisim.app/api/v1/intellisim`
  - Production: `https://intellisim.app/api/v1/intellisim`

**Usage in Code**:
```typescript
// src/shared/api/client.ts
const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1/intellisim',
})
```

#### VITE_WS_BASE_URL

- **Purpose**: Base URL for WebSocket connections (simulation streaming)
- **Type**: string (WebSocket URL)
- **Default**: `ws://localhost:8080/api/v1/intellisim`
- **Examples**:
  - Dev: `ws://localhost:8080/api/v1/intellisim`
  - Staging: `wss://staging.intellisim.app/api/v1/intellisim`
  - Production: `wss://intellisim.app/api/v1/intellisim`

**Usage in Code**:
```typescript
// src/modules/simulation/composables/useSimulationApi.ts
function getStreamUrl(id: string): string {
  const wsBase = import.meta.env.VITE_WS_BASE_URL || `ws://${window.location.host}/api/v1/intellisim`
  return `${wsBase}/simulations/${id}/stream`
}
```

**Note**: WebSocket URLs require `wss://` (secure) in production, `ws://` in development.

### Creating Custom Environment

To use different backend for development:

**Create `.env.development.local`** (not tracked):
```
VITE_API_BASE_URL=http://192.168.1.100:8080/api/v1/intellisim
VITE_WS_BASE_URL=ws://192.168.1.100:8080/api/v1/intellisim
```

Vite loads files in order: `.env` → `.env.{mode}` → `.env.{mode}.local`

---

## Development Server

### Start Dev Server

```bash
npm run dev
```

**Output**:
```
VITE v7.2.2  ready in 245 ms

➜  Local:   http://localhost:3000/
➜  press h to show help
```

**Server Properties**:
- **Port**: 3000 (configurable in vite.config.ts)
- **Hot Module Reload (HMR)**: Enabled (code changes auto-refresh)
- **Proxy**: Routes `/api` and `/ws` to backend
- **HTTPS**: Not enabled (use localhost only)

### Vite Configuration

**File**: `vite.config.ts`

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
})
```

### Proxy Configuration

Dev server proxies requests to backend:

| Pattern | Forward To | Purpose |
|---------|-----------|---------|
| `/api/*` | `http://localhost:8080/api/*` | REST API calls |
| `/ws/*` | `ws://localhost:8080/ws/*` | WebSocket connections |

**Why Proxy?**: Avoids CORS issues during development. In production, same-origin serving handles this.

**Changing Target**:

If backend is on different host (e.g., 192.168.1.100):

```typescript
server: {
  proxy: {
    '/api': {
      target: 'http://192.168.1.100:8080',
      changeOrigin: true,
    },
    '/ws': {
      target: 'ws://192.168.1.100:8080',
      ws: true,
    },
  },
}
```

Then restart dev server: `npm run dev`

### Accessing the App

1. Open http://localhost:3000 in browser
2. Should see login/welcome screen
3. App loads `/chat/new` by default (router redirect)

### Common Issues

| Issue | Solution |
|-------|----------|
| Port 3000 already in use | Kill process: `lsof -i :3000` → `kill -9 <PID>` or change port in vite.config.ts |
| API calls fail with 404 | Check backend running on :8080; check proxy config |
| WebSocket connection fails | Verify backend WebSocket endpoint; check firewall |
| Blank page / white screen | Check console for errors; clear cache (Ctrl+Shift+Delete) |
| Hot reload not working | Restart dev server; check file permissions |

---

## Build System

### Production Build

```bash
npm run build
```

**Steps**:
1. Type check: `vue-tsc -b` (validates TypeScript)
2. Build: `vite build` (bundles and minifies)

**Output**:
- Directory: `dist/`
- Files:
  - `index.html` — Entry point
  - `assets/` — JS, CSS, images (hashed filenames)
  - `assets/vendor.*.js` — Node modules (code-split)
  - `assets/main.*.js` — Application code

**Duration**: 30-60 seconds

**File Sizes** (example):
```
dist/index.html              2.5 KB
dist/assets/vendor.xxxxx.js  450 KB (gzipped: ~120 KB)
dist/assets/main.xxxxx.js    250 KB (gzipped: ~60 KB)
```

### Build Configuration

**File**: `vite.config.ts` (build section not shown, uses defaults)

**Default Behavior**:
- Target: ES2020+ (modern browsers)
- Minification: On (esbuild)
- Source maps: Not generated (production) / generated (dev)
- Code splitting: Automatic (node_modules separate)

### Type Checking

```bash
npm run build
```

Automatically runs `vue-tsc -b` before Vite build. If TypeScript errors found, build stops.

**Manual Type Check**:
```bash
npm run type-check    # Not defined in package.json, use:
npx vue-tsc --noEmit
```

### Preview Build

After building, preview production locally:

```bash
npm run preview
```

**Output**:
```
➜  Local:   http://localhost:4173/
```

Serves `dist/` folder as static files (similar to production).

**Use Case**: Test production build before deployment (validates HMR disabled, minification works, etc.).

---

## TypeScript Configuration

**File**: `tsconfig.app.json`

```typescript
{
  "extends": "@vue/tsconfig/tsconfig.dom.json",
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "types": ["vite/client"],
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    },
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "erasableSyntaxOnly": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedSideEffectImports": true
  },
  "include": ["src/**/*.ts", "src/**/*.tsx", "src/**/*.vue"]
}
```

### Key Settings

| Setting | Value | Purpose |
|---------|-------|---------|
| strict | true | Enable all type checks |
| noUnusedLocals | true | Error on unused variables |
| noUnusedParameters | true | Error on unused function parameters |
| noFallthroughCasesInSwitch | true | Error if switch case missing break |
| paths | `@/*` → `src/*` | Import alias resolution |

### Type Safety

All Vue components use `<script setup lang="ts">` with strict TypeScript:

```typescript
// ✅ Correct
interface Props {
  messages: ChatMessageResponse[]
  loading?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  loading: false,
})

// ❌ Wrong (not type-safe)
const props = defineProps({
  messages: Array,
  loading: Boolean,
})
```

---

## Deployment

### Static Hosting

The built `dist/` folder contains all necessary files for static hosting.

### Supported Hosts

- **Netlify**: Drag-drop `dist/` or connect Git repo
- **Vercel**: Git integration, auto-deploys
- **AWS S3 + CloudFront**: Upload dist to bucket, serve via CDN
- **GitHub Pages**: Use gh-pages action
- **Docker**: Nginx container serving dist/

### Pre-Deployment Checklist

- [ ] Build succeeds: `npm run build` (no errors)
- [ ] Type check passes: `npx vue-tsc --noEmit`
- [ ] Environment vars set (VITE_API_BASE_URL, VITE_WS_BASE_URL)
- [ ] Backend API URL correct for environment
- [ ] WebSocket URL uses wss:// in production
- [ ] No console errors: `npm run preview`
- [ ] Test critical flows: chat, model creation, simulation

### Docker Deployment Example

**Dockerfile**:
```dockerfile
FROM node:18-alpine as build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

**Build & Run**:
```bash
docker build -t intellisim-frontend .
docker run -p 80:80 intellisim-frontend
```

### Nginx Configuration

**nginx.conf** (for single-page app routing):
```nginx
server {
  listen 80;
  location / {
    root /usr/share/nginx/html;
    try_files $uri $uri/ /index.html;
  }
  location /api/ {
    proxy_pass http://backend:8080;
  }
}
```

---

## Performance Optimization

### Code Splitting

Routes lazy-loaded via dynamic imports:

```typescript
// In router.ts
component: () => import('@/modules/chat/views/ChatView.vue')
```

Each route component loaded only when needed (reduces initial bundle size).

### Asset Optimization

- **CSS**: Tailwind purges unused styles in production
- **Images**: Use modern formats (WebP with fallback)
- **Icons**: PrimeIcons loaded via CSS (no unused icons in bundle)
- **JavaScript**: esbuild minifies and tree-shakes

### Build Metrics

Check bundle size:

```bash
npm run build
# Check dist/ folder size:
du -sh dist/
# Detailed breakdown (install rollup-plugin-visualizer for detailed analysis)
```

Typical production build: ~500 KB (uncompressed), ~120 KB (gzipped).

---

## Development Workflow

### Typical Session

```bash
# 1. Start dev server
npm run dev

# 2. Edit files (auto-reload on save)
# Open http://localhost:3000

# 3. Before committing
npm run build    # Validate build
npx vue-tsc --noEmit  # Type check

# 4. Commit to Git
git add .
git commit -m "Feature: Add dark mode"
```

### Debugging

**Browser DevTools**:
1. Press F12 (or Cmd+Option+I on macOS)
2. Vue DevTools extension (Chrome/Firefox)
   - Install: Chrome Web Store / Firefox Add-ons
   - Inspect components, watch state changes
3. Network tab: Monitor API calls
4. Console tab: Check for errors

**VS Code Debugging**:
Install "Vite" extension, set breakpoints, debug in editor.

---

## Troubleshooting

### npm install Fails

```bash
# Clear cache and retry
npm cache clean --force
rm -rf node_modules package-lock.json
npm install
```

### Dev Server Won't Start

```bash
# Check port 3000 in use
lsof -i :3000

# Kill process
kill -9 <PID>

# Try different port (edit vite.config.ts)
npm run dev
```

### Build Fails

```bash
# Type errors
npx vue-tsc --noEmit

# Rebuild from scratch
rm -rf dist/ node_modules/.vite
npm run build
```

### API Calls Timeout

- Check backend is running: `curl http://localhost:8080/health`
- Check network tab in DevTools for actual requests
- Increase timeout in `src/shared/api/client.ts` (default 60s)

### WebSocket Connection Fails

- Verify backend supports WebSocket on `/api/v1/intellisim/simulations/:id/stream`
- Check firewall allows WebSocket protocol
- In production, ensure reverse proxy (Nginx, etc.) supports WebSocket upgrade

---

## Next Steps

- Read [Architecture Guide](./architecture.md) for project structure
- Read [API Integration Guide](./api-integration.md) for backend communication
- Read [Components Catalog](./components.md) for UI components
- Explore [Theme Customization](./theming.md) for styling
