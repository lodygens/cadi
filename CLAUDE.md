# CADI — consignes pour Claude et ses agents

CADI (Consommation Alerte Danger Informations) prévient les consommateurs, par email, des rappels de produits en France, et produit des statistiques sur ces rappels. Voir [README.md](README.md).

## Qui décide

- **L'utilisateur humain est le Product Owner** : il définit les features. Il est aussi l'expert architecture, sécurité, infra (Docker, Kubernetes) et développement (Go, C++, Java).
- **Il relit tout.** Aucun agent ne committe, ne merge, ne pousse, ne publie, ne déploie ni ne supprime de données sans son accord explicite.
- En cas de doute sur le besoin : **poser la question** plutôt que deviner. Une ambiguïté dans la spec se signale ; elle ne se tranche pas en silence.

## Sources de vérité (par ordre de priorité)

1. La demande explicite du PO dans la conversation en cours.
2. [doc/specifications.md](doc/specifications.md) : fonctionnalités, RGPD, sécurité, modèle de données, workflows, architecture, plan de test, risques.
3. `doc/adr/` : décisions d'architecture **acceptées** par le PO (format décrit dans l'agent `architecte`).
4. `doc/api/openapi.yaml` : contrat de l'API REST, écrit par l'architecte ; le code serveur et le client TypeScript en sont générés.
5. [doc/umldiagrams/](doc/umldiagrams/) : diagrammes PlantUML (`.uml`) et leurs images (`.png`) ; un diagramme par workflow dans `workflows/`.
6. Le code et les tests existants.
7. Tes propres hypothèses, toujours signalées comme telles.

Le code doit suivre la spec. Si deux sources se contredisent, c'est un défaut à signaler, pas une liberté : ne choisis jamais en silence celle qui arrange l'implémentation.

## Mode de travail

- **Lecture seule** quand la demande est de relire, analyser, auditer, comparer, expliquer ou vérifier : on ne modifie aucun fichier et on ne « corrige pas au passage ». On distingue constats, risques, hypothèses et recommandations.
- **Écriture** quand la demande est de concevoir, implémenter, corriger, créer ou modifier : uniquement dans le périmètre demandé et dans celui de l'agent concerné.

## Stack technique (décidée dans la spec)

- **Backend** : Go. Composants faiblement couplés : API REST (contrat OpenAPI), logique métier, service lecture (collecte des rappels), service base de données (seul autorisé à lire et écrire la base), service notification (emails).
- **Base de données** : PostgreSQL.
- **Frontend** : SPA React + TypeScript, construite avec Vite ; fichiers statiques en production ; même domaine que l'API ; aucun service tiers ; français ; mobile d'abord ; accessibilité RGAA (WCAG AA).
- **Infra** : conteneurs Docker, déploiement Docker compose, hébergement OVH en France.
- **Objectif de charge (v1)** : 100 000 utilisateurs inscrits, 1 000 simultanés, pics de consultation absorbés par le cache ; un seul serveur. Seule l'API peut tourner en plusieurs conteneurs ; le service lecture et le service notification ont chacun une seule instance.

## État du dépôt

- Le projet sort de la phase de spécification.
- [src/](src/) contient un **prototype CLI** Go (interrogation de l'API RappelConso). Ce n'est pas l'application cible ; ne pas le faire évoluer sans demande explicite.
- L'arborescence de l'application cible est à fixer par un ADR. Proposition par défaut, tant qu'aucun ADR ne dit autre chose :

```
backend/      code Go de l'application        (agent backend-go)
frontend/     SPA React + TypeScript          (agent frontend)
deploy/       Dockerfiles, docker compose     (agent devops)
.github/      CI                              (agent devops)
doc/adr/      décisions d'architecture        (agent architecte)
doc/api/      contrat OpenAPI                 (agent architecte)
doc/tests/    stratégie et couverture de test (agent qa)
doc/securite/ rapports et modèle de menace    (agent securite)
```

## L'équipe d'agents (`.claude/agents/`)

| Agent | Rôle | Modifie |
|---|---|---|
| `architecte` | Conçoit : ADR, critères d'acceptation, diagrammes, interfaces (contrat OpenAPI, modèle SQL) ; vérifie la conformité ; met la doc à jour en fin de cycle | `doc/` (hors `doc/tests/` et `doc/securite/`), `README.md` |
| `qa` | Stratégie de test, tests écrits **avant** le code (TDD), tests d'intégration et E2E, couverture des critères d'acceptation | fichiers de test, `doc/tests/` |
| `backend-go` | Pose le squelette puis implémente le backend Go | `backend/` (hors tests écrits par QA) |
| `frontend` | Pose le squelette puis implémente la SPA React | `frontend/` (hors tests écrits par QA) |
| `devops` | Docker, docker compose, CI/CD | `deploy/`, `.github/`, `Makefile` |
| `securite` | Revue de conception, audit, fuzzing, analyse de dépendances, modèle de menace | tests de fuzzing, `doc/securite/` |

### Organisation git

- Le dépôt doit être sous git : la review se fait sur des diffs.
- **Une branche par feature** (`feature/<nom-court>`), créée par le PO à partir de `main`. Aucun agent ne travaille sur `main`.
- Tous les agents travaillent **dans la copie courante, sur cette branche**. Il n'y a pas de worktree : chaque étape voit directement le travail des étapes précédentes, sans merge intermédiaire.
- Le parallélisme ne vient pas de copies séparées mais du **partage des fichiers** : deux agents ne sont lancés en même temps que s'ils n'écrivent pas dans les mêmes dossiers (ex : `backend-go` et `frontend`).
- Chaque **point de validation** se fait sur `git diff` depuis le point précédent. Le PO committe s'il valide ; l'étape suivante part de ce commit.
- **Budget de revue** : le diff d'un point de validation reste lisible, environ 400 lignes écrites à la main (code généré exclu). Si l'ADR prévoit plus, l'architecte découpe la feature en **lots**, chacun livré par son propre cycle et sa propre PR.
- `.claude/settings.json` demande une confirmation avant tout `git commit`, `git push` ou `gh pr create`, et interdit les commandes destructrices (`git reset --hard`, `git clean`, push forcé).

### Cycle d'une feature

1. Le PO décrit la feature dans la spec et crée la branche.
2. `architecte` produit un ADR (statut « proposé ») : critères d'acceptation numérotés (`CA1`, `CA2`…), options étudiées, interfaces exactes (signatures Go, contrat OpenAPI, tables SQL), traçabilité, découpage en tâches.
3. *(Si la feature touche à l'authentification, aux sessions, aux données personnelles ou à des entrées externes.)* `securite` fait une **revue de conception** de l'ADR proposé.
4. **Validation du PO** : il passe l'ADR à « accepté ». Aucun agent ne démarre l'étape 5 sur un ADR qui n'est pas accepté.
5. **Squelette** : `backend-go` (et `frontend` si concerné) crée les types, interfaces et fonctions de l'ADR, dont le corps renvoie seulement `ErrNotImplemented` (ou lève une erreur « non implémenté » en TypeScript). Le code compile, aucune logique → **validation du PO**.
6. `qa` écrit les tests contre ce squelette, chacun rattaché à un critère d'acceptation : ils **compilent et échouent** à l'exécution → **validation du PO**.
7. `backend-go` et `frontend` implémentent, en parallèle, chacun dans ses fichiers, jusqu'à ce que les tests passent.
8. `securite` audite et fuzze la branche → **le PO décide** des corrections, confiées à l'agent concerné.
9. `devops` met à jour l'image, le déploiement et la CI.
10. `architecte` vérifie la conformité du code à l'ADR et met à jour la documentation (diagrammes, README) → **validation du PO**.
11. Le PO relit la PR complète et merge dans `main`.

Règles communes à tous les agents :

- **Vérifier avant d'écrire** : branche de feature (jamais `main`), ADR accepté (sauf pour l'architecte), état de `git status` cohérent avec l'étape précédente. Sinon, s'arrêter et le dire.
- **Rester dans son périmètre de fichiers.** Si un changement hors périmètre est nécessaire, le décrire dans le rapport final au lieu de le faire.
- **Ne jamais affaiblir un test** pour le faire passer, ni désactiver un contrôle (lint, sécurité, CI).
- **Ne jamais annoncer qu'une commande a réussi sans l'avoir lancée.** Une commande non lancée est signalée avec la raison.
- **Aucun secret** dans le dépôt, les logs, les tests ou les exemples (en particulier : l'email de l'administrateur, voir la section Sécurité de la spec).
- **Données personnelles** : appliquer la section Conformité (RGPD) de la spec ; minimiser ; ne jamais logger d'adresse email en clair.
- Les textes issus des sources de rappels sont **non fiables** (risque XSS, injection) : toujours les traiter comme des données.
- Terminer chaque tâche par un **rapport pour le reviewer** (format défini dans chaque agent) : ce qui a été fait, les critères d'acceptation couverts, les choix non prévus, les risques, ce qu'il faut relire en priorité.

## Conventions

- Langue : documentation, commentaires et messages de commit en **français** ; identifiants de code en anglais.
- Commits : petits, un sujet par commit, message à l'impératif (`Ajoute la création d'alerte`).
- Ne pas committer ni pousser sans demande du PO. Un sous-agent ne committe jamais.
- Diagrammes : PlantUML ; tout `.uml` modifié doit avoir son `.png` régénéré.
