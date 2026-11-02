# 20261030. Socle de l'application, lot 0b : frontend minimal et CI frontend

- Statut : proposé
- Date : 2026-10-30
- Feature / section de la spec : [Périmètre technique](../specifications.md#périmètre-technique), [Frontend](../specifications.md#frontend), [Plan de test](../specifications.md#plan-de-test), [Gestion des risques](../specifications.md#gestion-des-risques)
- Lot : 0b sur 2 (socle). **Dépend du lot 0a** ([20261030-socle](20261030-socle.md)) : contrat `doc/api/openapi.yaml`, `GET /api/sante` et compose de développement livrés.

## Historique des révisions

| Version | Date | Changements |
|---|---|---|
| 0.1 | 2026-10-30 | Première version : partie frontend de l'ADR 20261030-socle v0.2 (D8 côté TypeScript, D9, CA17–CA20, tâches T2, T4, T6, CI frontend), séparée en lot propre ; `npm ci --ignore-scripts` (revue [`doc/securite/2026-10-30-revue-adr-socle.md`](../securite/2026-10-30-revue-adr-socle.md), S11.3). |

## Remplace / modifie

Aucun.

## Contexte

Le lot 0a livre l'API `GET /api/sante` (`200 {"statut":"ok"}` ou `503 {"statut":"indisponible"}`, `default` : schéma `Error`) et le compose de développement. Ce lot pose la SPA minimale, sa génération de types à partir du contrat, ses tests et sa CI, et livre un comportement démontrable : **la page d'accueil affiche un message clair quand le service est indisponible**.

### Faits vérifiés

| # | Fait | Source |
|---|---|---|
| F1 | SPA React + TypeScript (Vite), fichiers statiques en production, même domaine que l'API, aucun service tiers, français, mobile d'abord, RGAA (WCAG AA). | spec, *Frontend* |
| F2 | Backend indisponible → « message d'erreur clair ». | spec, *Gestion des risques* |
| F3 | Tests : Vitest + React Testing Library, axe ; Playwright + Mailpit en E2E. | spec, *Plan de test* |
| F4 | Contrat OpenAPI 3.0.3 `doc/api/openapi.yaml`, client TypeScript généré. | spec, *Interfaces* ; ADR 20261030-socle, D8 |
| F5 | `/api/sante` répond en moins de 3 s (`200` ou `503`) ; son état peut retarder de ~70 s sur un arrêt de PostgreSQL. | ADR 20261030-socle, CA2, D9 |
| F6 | Node.js 24.22.2 installé par nvm (le `node` par défaut du shell est la v10 : `nvm use` lira `frontend/.nvmrc`). | `~/.nvm/versions/node` |
| F7 | Agent `frontend` : scripts `generate:api`, `typecheck`, `lint`, `test`, `build`. | `.claude/agents/frontend.md` |

## Critères d'acceptation

- **CA1** Étant donné l'API qui répond `200 {"statut":"ok"}`, quand l'utilisateur ouvre la page d'accueil, alors il voit le titre de niveau 1 « CADI », une courte présentation, et aucun message d'indisponibilité.
- **CA2** Étant donné l'API qui répond `503`, un autre code (dont `500 {"code":"erreur_interne"}`), une erreur réseau, aucune réponse en 5 s, ou un corps non conforme, quand l'utilisateur ouvre la page d'accueil, alors il voit dans une zone `role="alert"` : « Le service est momentanément indisponible. Réessayez plus tard. ». Tant qu'aucune réponse n'est arrivée (état inconnu), ce message n'est pas affiché.
- **CA3** Étant donné la page d'accueil dans chacun des états inconnu, `ok` et indisponible, quand on la vérifie avec axe (WCAG 2.x A et AA), alors aucune violation n'est signalée.
- **CA4** Étant donné la construction de production (`npm run build`), quand on inspecte `dist/`, alors `index.html` ne contient aucun script ni style en ligne, et aucun fichier ne référence une ressource d'un autre domaine.
- **CA5** Étant donné le compose du lot 0a démarré (`make dev-secrets`, `make dev-up`) et `npm run dev` dans `frontend/`, quand on ouvre `http://localhost:5173/`, alors aucun message d'indisponibilité n'est affiché ; quand on arrête le conteneur `store`, alors un rechargement 10 s plus tard affiche le message de CA2 ; quand on le relance, puis qu'on arrête `postgres`, alors un rechargement 75 s plus tard l'affiche ; quand on relance `postgres`, un rechargement 75 s plus tard ne l'affiche plus.

## Options étudiées

### O1. Génération du client TypeScript

| Option | Avantages | Coûts / risques |
|---|---|---|
| **A. openapi-typescript (types) + openapi-fetch (client).** | Cités par la spec ; client minuscule, typé par le contrat. | Deux dépendances. |
| B. Client complet généré (ex : openapi-generator). | Tout est généré. | Java à l'outillage, beaucoup de code généré. |
| C. `fetch` écrit à la main. *La plus simple en outils.* | Aucune dépendance. | Contraire à la spec ; écarts silencieux avec le contrat. |

### O2. Scripts d'installation npm en CI

| Option | Avantages | Coûts / risques |
|---|---|---|
| **A. `npm ci --ignore-scripts`.** | Aucun script de dépendance exécuté avec le dépôt accessible. | Une dépendance qui a besoin de son script (ex : binaire d'esbuild) pourrait casser la construction : à vérifier en T4. |
| B. `npm ci`. | Rien à vérifier. | Exécute le code d'installation de toute dépendance transitive. |

## Décision

**[D]** décidé par cet ADR (sous réserve d'acceptation), **[R]** reporté.

### D1. Frontend minimal [D]

- `frontend/` : Node.js 24 (`.nvmrc` = `24`, `"engines": {"node": ">=24"}`), npm, `package-lock.json` committé, TypeScript `strict`.
- Dépendances : `react`, `react-dom`, `openapi-fetch` ; développement : `vite`, `@vitejs/plugin-react`, `typescript`, `openapi-typescript`, `vitest`, `jsdom`, `@testing-library/react`, `axe-core`, `eslint`, `typescript-eslint`, `eslint-plugin-react-hooks`, `eslint-plugin-jsx-a11y`. Pas de routeur ; Playwright au lot de la connexion [R].
- Scripts : `dev` (`vite`, port 5173, proxy `/api` → `http://127.0.0.1:8080`), `build` (`tsc --noEmit && vite build`), `preview`, `typecheck`, `lint` (`eslint .`), `test` (`vitest run`), `generate:api` (`openapi-typescript ../doc/api/openapi.yaml -o src/api/schema.d.ts`).
- `src/api/schema.d.ts` généré, committé, jamais modifié à la main ; vérifié par `make check-generated` (étendu par devops).
- `index.html` : `lang="fr"`, aucun script ni style en ligne, aucune ressource d'un autre domaine (CA4).

### D2. État de santé affiché [D]

États : `unknown` (rien affiché), `ok`, `unavailable` (`503`, tout autre code, erreur réseau, 5 s sans réponse, corps hors contrat). Délai de 5 s supérieur au délai maximal de l'API (3 s). Une seule vérification à l'ouverture de la page ; pas de rafraîchissement périodique.

### D3. CI frontend [D]

Job **frontend** ajouté à `.github/workflows/ci.yml` (mêmes règles que le lot 0a : `contents: read`, actions épinglées par SHA, `persist-credentials: false`) : `actions/setup-node` (version de `.nvmrc`), `npm ci --ignore-scripts` (O2-A ; si la construction échoue sans les scripts, devops le signale et le PO décide), `typecheck`, `lint`, `test`, `build`, `npm audit --audit-level=high`. `make check-generated` couvre `schema.d.ts`. `.gitignore` : `frontend/node_modules/`, `frontend/dist/`.

## Conséquences

**Positives** : le frontend consomme le contrat généré dès le premier écran ; l'indisponibilité est montrée sans faux négatif (état inconnu distinct).

**Négatives** : ~520 lignes écrites à la main pour un seul écran ; l'affichage peut retarder d'environ 70 s sur un arrêt de PostgreSQL (lot 0a, D9).

**Risques** : `--ignore-scripts` pourrait empêcher la construction (O2).

## Impacts sécurité et RGPD

- Aucune donnée personnelle, aucun cookie, aucune ressource tierce ni script en ligne (CA4).
- Le message d'indisponibilité ne reprend jamais le contenu d'une réponse.
- CSP et en-têtes de sécurité : au serveur web de production [R, lot de production].
- Revue de conception par `securite` non requise (ni authentification, ni donnée personnelle, ni entrée externe) ; `npm audit` en CI.

## Interfaces

Toutes les fonctions du squelette lèvent `new Error("non implémenté")`.

```ts
// src/api/client.ts
export const api: ReturnType<typeof createClient<paths>>; // createClient<paths>({ baseUrl: "/api" })
// src/api/health.ts
export type HealthState = "ok" | "unavailable";
export const HEALTH_TIMEOUT_MS = 5000;
/** Ne rejette jamais : erreur, code ≠ 200, corps hors contrat ou délai → "unavailable". */
export function fetchHealth(): Promise<HealthState>;
// src/components/ServiceStatus.tsx
export interface ServiceStatusProps { check?: () => Promise<HealthState> } // défaut : fetchHealth
export function ServiceStatus(props: ServiceStatusProps): ReactElement | null;
// inconnu/ok → null ; unavailable → <p role="alert">Le service est momentanément indisponible. Réessayez plus tard.</p>
// src/pages/HomePage.tsx
export function HomePage(): ReactElement; // <main> : <h1>CADI</h1>, présentation, <ServiceStatus />
// src/App.tsx, src/main.tsx : montent <HomePage /> dans #root.
```

## Traçabilité

| CA | Décision / interface | Test prévu (écrit par qa) |
|---|---|---|
| CA1, CA2 | D2, `HomePage`, `ServiceStatus`, `fetchHealth` | Vitest + RTL, `check` et `fetch` simulés ; état inconnu ; délai (horloge simulée) |
| CA3 | D1, D2 | Vitest + axe-core, chaque état |
| CA4 | D1 | test Vitest sur `dist/` après `build` (ou script de CI, à choisir par qa) |
| CA5 | D2, lot 0a D10 | démonstration du PO (T5) ; E2E Playwright au lot de la connexion |

## Découpage en tâches

Tailles : lignes écrites à la main (code généré et `package-lock.json` exclus), estimées.

| Tâche | Agent | Fichiers | CA | Dépend de | Critère de fin | Taille |
|---|---|---|---|---|---|---|
| T1 | frontend | `frontend/` (D1, Interfaces), `src/api/schema.d.ts` généré | structure | lot 0a fusionné, acceptation | `generate:api`, `typecheck`, `build` passent | ~200 |
| T2 | qa | `frontend/src/**/*.test.ts(x)` ; `doc/tests/20261030-socle-frontend.md` | CA1–CA4 | T1 | tests compilés, en échec sur « non implémenté » | ~140 |
| T3 | frontend | `frontend/` hors tests | CA1–CA4 | T2 | `typecheck`, `lint`, `test`, `build` passent | ~80 |
| T4 | devops | `.github/workflows/ci.yml` (job frontend), `Makefile` (`check-generated`), `.gitignore` | D3 | T3 | `actionlint` passe ou absence signalée ; `npm ci --ignore-scripts` puis `build` vérifiés | ~60 |
| T5 | architecte (+ PO) | conformité ; `README.md` ; `doc/umldiagrams/frontend.uml` et `.png` si besoin | CA5 (démonstration) | T4 | rapport de conformité ; doc à jour | ~40 |

**Points de validation** : T1 → T2 → T3 → T4 → T5. Aucun ne dépasse ~400 lignes.

**Estimation du lot 0b** : ~520 lignes écrites à la main (dont ~140 de tests), en une PR.

## Points reportés

- Playwright et Mailpit en E2E, routeur → lot de la connexion.
- Image et serveur web du frontend, CSP, en-têtes de sécurité → lot du déploiement de production.
- À revoir si : la page d'accueil doit rafraîchir l'état périodiquement.

## Questions ouvertes pour le PO

Aucune.
