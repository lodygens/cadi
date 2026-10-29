# CADI — consignes pour Claude et ses agents

CADI (Consommation Alerte Danger Informations) prévient les consommateurs, par email, des rappels de produits en France, et produit des statistiques sur ces rappels. Voir [README.md](README.md).

## Qui décide

- **L'utilisateur humain est le Product Owner** : il définit les features. Il est aussi l'expert architecture, sécurité, infra (Docker, Kubernetes) et développement (Go, C++, Java).
- **Il relit tout.** Aucun agent ne merge, ne pousse, ne publie, ne déploie ni ne supprime de données sans son accord explicite.
- En cas de doute sur le besoin : **poser la question** plutôt que deviner. Une ambiguïté dans la spec se signale ; elle ne se tranche pas en silence.

## Sources de vérité (par ordre de priorité)

1. [doc/specifications.md](doc/specifications.md) : fonctionnalités, RGPD, sécurité, modèle de données, workflows, architecture, plan de test, risques.
2. `doc/adr/` : décisions d'architecture **validées** par le PO (format décrit dans l'agent `architecte`).
3. `doc/api/openapi.yaml` : contrat de l'API REST, écrit par l'architecte ; le code serveur et le client TypeScript en sont générés.
4. [doc/umldiagrams/](doc/umldiagrams/) : diagrammes PlantUML (`.uml`) et leurs images (`.png`) ; un diagramme par workflow dans `workflows/`.

Le code doit suivre la spec. Si le code et la spec divergent, c'est un défaut à signaler, pas une liberté.

## Stack technique (décidée dans la spec)

- **Backend** : Go. Composants faiblement couplés : API REST (contrat OpenAPI), logique métier, service lecture (collecte des rappels), service base de données (seul autorisé à lire et écrire la base), service notification (emails).
- **Base de données** : PostgreSQL.
- **Frontend** : SPA React + TypeScript, construite avec Vite ; fichiers statiques en production ; même domaine que l'API ; aucun service tiers ; français ; mobile d'abord ; accessibilité RGAA (WCAG AA).
- **Infra** : conteneurs Docker, déploiement Kubernetes, hébergement OVH en France.
- **Objectif de charge** : 100 000 utilisateurs simultanés.

## État du dépôt

- Le projet sort de la phase de spécification.
- [src/](src/) contient un **prototype CLI** Go (interrogation de l'API RappelConso). Ce n'est pas l'application cible ; ne pas le faire évoluer sans demande explicite.
- L'arborescence de l'application cible est à fixer par un ADR. Proposition par défaut, tant qu'aucun ADR ne dit autre chose :

```
backend/      code Go de l'application        (agent backend-go)
frontend/     SPA React + TypeScript          (agent frontend)
deploy/       Dockerfiles, manifestes k8s     (agent devops)
.github/      CI                              (agent devops)
doc/adr/      décisions d'architecture        (agent architecte)
doc/tests/    stratégie de test               (agent qa)
doc/securite/ rapports et modèle de menace    (agent securite)
```

## L'équipe d'agents (`.claude/agents/`)

| Agent | Rôle | Modifie |
|---|---|---|
| `architecte` | Conçoit : ADR, diagrammes, interfaces (contrat OpenAPI, modèle SQL) | `doc/` uniquement |
| `qa` | Stratégie de test, tests écrits **avant** le code (TDD), tests d'intégration et E2E | fichiers de test, `doc/tests/` |
| `backend-go` | Pose le squelette puis implémente le backend Go | `backend/` (hors tests écrits par QA) |
| `frontend` | Pose le squelette puis implémente la SPA React | `frontend/` (hors tests écrits par QA) |
| `devops` | Docker, Kubernetes, CI/CD | `deploy/`, `.github/`, `Makefile` |
| `securite` | Audit, fuzzing, analyse de dépendances, modèle de menace | tests de fuzzing, `doc/securite/` |

### Organisation git

- Le dépôt doit être sous git : la review se fait sur des diffs.
- **Une branche par feature** (`feature/<nom-court>`), créée par le PO à partir de `main`.
- Tous les agents travaillent **dans la copie courante, sur cette branche**. Il n'y a pas de worktree : chaque étape voit directement le travail des étapes précédentes, sans merge intermédiaire.
- Le parallélisme ne vient pas de copies séparées mais du **partage des fichiers** : deux agents ne sont lancés en même temps que s'ils n'écrivent pas dans les mêmes dossiers (ex : `backend-go` et `frontend`).
- Chaque **point de validation** se fait sur `git diff` depuis le point précédent. Le PO committe s'il valide ; l'étape suivante part de ce commit.

### Cycle d'une feature

1. Le PO décrit la feature dans la spec et crée la branche.
2. `architecte` produit un ADR (statut « proposé ») avec les interfaces exactes (signatures Go, contrat OpenAPI, tables SQL) → **validation du PO**.
3. **Squelette** : `backend-go` (et `frontend` si concerné) crée les types, interfaces et fonctions de l'ADR, dont le corps renvoie seulement `ErrNotImplemented` (ou lève une erreur « non implémenté » en TypeScript). Le code compile, aucune logique → **validation du PO**.
4. `qa` écrit les tests contre ce squelette : ils **compilent et échouent** à l'exécution → **validation du PO**.
5. `backend-go` et `frontend` implémentent, en parallèle, chacun dans ses fichiers, jusqu'à ce que les tests passent.
6. `securite` audite et fuzze la branche → **le PO décide** des corrections, confiées à l'agent concerné.
7. `devops` met à jour l'image, le déploiement et la CI.
8. Le PO relit la PR complète et merge dans `main`.

Règles communes à tous les agents :

- **Rester dans son périmètre de fichiers.** Si un changement hors périmètre est nécessaire, le décrire dans le rapport final au lieu de le faire.
- **Ne jamais affaiblir un test** pour le faire passer, ni désactiver un contrôle (lint, sécurité, CI).
- **Aucun secret** dans le dépôt, les logs, les tests ou les exemples (en particulier : l'email de l'administrateur, voir la section Sécurité de la spec).
- **Données personnelles** : appliquer la section Conformité (RGPD) de la spec ; minimiser ; ne jamais logger d'adresse email en clair.
- Les textes issus des sources de rappels sont **non fiables** (risque XSS, injection) : toujours les traiter comme des données.
- Terminer chaque tâche par un **rapport pour le reviewer** (format défini dans chaque agent) : ce qui a été fait, les choix non prévus, les risques, ce qu'il faut relire en priorité.

## Conventions

- Langue : documentation, commentaires et messages de commit en **français** ; identifiants de code en anglais.
- Commits : petits, un sujet par commit, message à l'impératif (`Ajoute la création d'alerte`).
- Ne pas committer ni pousser sans demande du PO.
- Diagrammes : PlantUML ; tout `.uml` modifié doit avoir son `.png` régénéré.
