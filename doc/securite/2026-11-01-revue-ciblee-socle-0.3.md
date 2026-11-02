# Revue de conception ciblée — ADR 20261030 « Socle, lot 0a », version 0.3

- Date : 2026-11-01
- Branche : `feature/socle`
- Objet : `doc/adr/20261030-socle.md` (v0.3, statut « proposé »), `doc/api/store.yaml` (v0.2.0), `doc/specifications.md` (modifications non committées de *Backend > Architecture*, composant 3, et *Déploiement*)
- Type : revue de conception ciblée sur les ajouts de la v0.3 (aucun code n'existe ; aucun outil d'analyse lancé)
- Revue précédente : [`2026-10-30-revue-adr-socle.md`](2026-10-30-revue-adr-socle.md) (S1–S16), dont les constats ne sont pas répétés ici
- Référentiel : spec *Backend > Architecture* (l. 1007), *Instances* (l. 1010), *Déploiement* (l. 1012), *Gouvernance des rappels* (l. 331-333), *Sécurité* (l. 203-224) ; OWASP ASVS 4.0.3 niveau 2 ; OWASP API Security Top 10 2023.

## Périmètre

1. Authentification et autorisation de l'interface interne du store (D7, CA9–CA12, CA29, `store.yaml`).
2. Rôles PostgreSQL `cadi_owner` / `cadi_app`, script d'initialisation, `EnsureSchema`, refus du superutilisateur (D6, CA21–CA28).
3. Santé : moniteur, fichier de santé, `healthcheck`, cache de l'API, `/api/sante` (D9, CA1–CA4, CA17–CA20).
4. Réseaux par appelant et durcissement des conteneurs (D10, CA30, CA31).
5. Cohérence ADR ↔ spec sur la distribution des mots de passe.

## Synthèse

La v0.3 traite correctement S1, S2, S3, S6 et S8 : jeton par service, refus par défaut, `/sante` du store authentifiée sans casser le `healthcheck`, aucune requête anonyme jusqu'à PostgreSQL, deux rôles non superutilisateurs, réseaux par appelant. Aucun constat critique, haut ni moyen. Les constats portent sur l'écart entre ce que l'ADR affirme et ce que les CA permettent de vérifier, et sur deux détails de mise en œuvre que le socle fige pour tous les lots (point d'application de l'autorisation, contrôle des rôles au démarrage).

| # | Gravité | Sujet |
|---|---|---|
| S17 | basse | Autorisation appliquée par l'intergiciel strict : après le décodage des paramètres et du corps, et sur un nom d'opération qui n'est pas l'`operationId` du contrat |
| S18 | basse | Le contrôle au démarrage ne vérifie que `rolsuper` : appartenances à des rôles (`pg_execute_server_program`, `cadi_owner`) et autres attributs non vérifiés ; CA27 incomplet |
| S19 | basse | Le mot de passe de `cadi_owner` reste dans le store pendant toute sa vie : la séparation DDL/DML ne protège que contre l'injection SQL |
| S20 | basse | Mots de passe des rôles exposés aux traces de PostgreSQL par `CREATE ROLE … PASSWORD` ; aucun CA sur les traces de `postgres` ; rotation non décrite |
| S21 | basse | CA30 et CA31 incomplets : notifier non testé, appartenance au réseau `db` non vérifiée, conteneur `postgres` non durci, options du `tmpfs` |
| S22 | basse | Table des autorisations dupliquée en texte libre dans `store.yaml`, sans vérification ; `DefaultPermissions` modifiable à l'exécution |
| S23 | info | *Write once* par les droits : à étendre aux tables de liens, aux produits et magasins, et aux privilèges par défaut des fonctions |
| S24 | info | Cache de santé : attente sur mutex non annulable par la requête |
| S25 | info | Cohérence ADR ↔ spec : F1 et Q1 obsolètes depuis la précision de la spec |

---

## S17 — Point d'application de l'autorisation du store — basse

**Gravité** : basse. Le refus par défaut reste sûr (aucun contournement identifié), mais le mécanisme choisi traite l'entrée d'un appelant non autorisé avant de le refuser, et la clé de la table risque de ne jamais correspondre au nom reçu, ce qui pousse à un « correctif » permissif.

**Emplacement** :
- `doc/adr/20261030-socle.md:259` — « autorisation : intergiciel strict (reçoit l'`operationID`) qui applique `store.Permissions` (`operationId → services`) » ;
- `doc/adr/20261030-socle.md:503-505` — clé `"getStoreHealth"` (forme du contrat) ;
- `doc/adr/20261030-socle.md:60` (CA9) — l'événement trace le « chemin de route » ; `:204` (D4) — « jamais le chemin brut » ;
- `doc/adr/20261030-socle.md:61` (CA10).

**Constats** :
1. *Ordre.* Dans le code généré par oapi-codegen (`std-http-server` + `strict-server`), la liaison des paramètres est faite par le `ServerInterfaceWrapper`, puis le gestionnaire strict décode le corps JSON, **puis seulement** applique les `StrictMiddlewareFunc`. Un appelant authentifié mais non autorisé (ex : un collector compromis par une fiche RappelConso, qui appelle une opération réservée à l'API) fait donc lier ses paramètres et décoder jusqu'à 1 Mio de JSON par le store, et reçoit `400 requete_invalide` au lieu de `403` si sa requête est mal formée : la réponse dépend de la validité de l'entrée avant la décision d'autorisation. CA10 ne précise pas ce cas.
2. *Nom de l'opération.* Dans oapi-codegen, le nom transmis aux intergiciels stricts est le champ `OperationId` du générateur, qui vaut `ToCamelCase(operationId)` (vérifié dans `deepmap/oapi-codegen@v1.6.0`, `pkg/codegen/operations.go:437`, seule version présente dans le cache local ; à confirmer sur la v2 épinglée) : l'intergiciel recevrait `GetStoreHealth`, la table contient `getStoreHealth`. Effet : tout est refusé (sûr), CA11 échoue, et le correctif le plus rapide (comparaison insensible à la casse, double clé, repli) affaiblit la règle. CA12 (contrat ↔ table) passe malgré l'écart.
3. *Trace du refus `401`.* L'authentification est « externe au routeur » (`:258`) : pour un `401`, le gabarit de route n'est pas encore connu. « Chemin de route » (CA9) se lira comme le chemin brut, contraire à D4 et contrôlé par l'appelant.

**Correction proposée** (architecte, puis backend-go) : appliquer l'autorisation dans la même enveloppe HTTP que l'authentification, avant le routeur, à partir du gabarit : `_, pattern := mux.Handler(r)` (`net/http` ≥ 1.22, sans servir la requête) → table `gabarit → operationId` construite depuis le contrat (le test de conformité CA12 vérifie la correspondance) → `store.Permissions`. Gabarit inconnu → `404` pour un appelant authentifié, `401` sinon (inchangé). La trace de refus porte le gabarit ou `inconnue`, jamais le chemin brut. Si l'intergiciel strict est gardé, fixer explicitement la forme de la clé (celle du code généré) et ajouter à CA12 un test qui appelle réellement chaque opération du contrat par le handler et vérifie que la décision vient de la table. Étendre CA10 : « … y compris avec des paramètres ou un corps invalides : `403`, corps non lu ».

**Exigences** : ASVS V4.1.3, V4.1.5 (refus sûr, appliqué avant tout traitement), V7.1.x ; API5:2023 ; spec l. 1007 (« toute autre opération lui est refusée »).

---

## S18 — Contrôle des rôles au démarrage limité à `rolsuper` — basse

**Gravité** : basse. Les rôles sont créés par un script du dépôt relu par le PO, d'où une probabilité faible ; mais l'ADR affirme un moindre privilège garanti (`:327`) que les CA ne vérifient pas, et l'écart couvre précisément le scénario de S1 (`COPY … TO PROGRAM`).

**Emplacement** : `doc/adr/20261030-socle.md:242` (étape 2 : `rolsuper` seulement), `:86` (CA26), `:87` (CA27), `:235-236`, `:327` ; spec l. 1007 (« rôles ordinaires »).

**Scénarios** :
1. `GRANT pg_execute_server_program TO cadi_app` (ou `pg_write_server_files`, `pg_read_server_files`, `pg_read_all_data`, `pg_write_all_data`) : le rôle n'est pas superutilisateur, CA26 passe, et une injection SQL dans une opération future obtient l'exécution de commandes dans `postgres` ou l'écriture de toutes les tables, *write once* compris.
2. `GRANT cadi_owner TO cadi_app` : `NOINHERIT` empêche l'usage implicite, pas `SET ROLE cadi_owner` (option `SET` vraie par défaut, PostgreSQL ≥ 16) ; une injection fait `SET ROLE cadi_owner; DROP TABLE …`. CA27 se connecte en `cadi_app` sans `SET ROLE` et passe.
3. `CREATEROLE`, `CREATEDB`, `REPLICATION`, `BYPASSRLS` ajoutés par erreur : non détectés.

**Correction proposée** (architecte, puis backend-go et qa) : étape 2 de `EnsureSchema` : pour `cadi_owner` et `cadi_app`, refuser (`ErrSuperuser` élargi, ou `ErrPrivilegedRole`, code `2`) si l'un de `rolsuper`, `rolcreaterole`, `rolcreatedb`, `rolreplication`, `rolbypassrls` est vrai, ou si `pg_auth_members` contient une ligne dont le membre est l'un des deux rôles. Étendre CA26 à ces cas (au moins `pg_execute_server_program` et `cadi_owner` accordés à `cadi_app`) et CA27 à `SET ROLE cadi_owner`, `CREATE SCHEMA`, `CREATE TEMP TABLE`, `CREATE FUNCTION`, refusés pour `cadi_app`.

**Exigences** : spec l. 1007 ; ASVS V1.2.1, V1.2.2, V4.1.3.

---

## S19 — Mot de passe de `cadi_owner` présent dans le store en marche — basse

**Gravité** : basse. Le gain de O5-B est réel contre l'injection SQL par le pool ; il est nul contre une compromission du processus store, qui est le seul composant ayant accès réseau à PostgreSQL. L'ADR ne nomme pas cette limite.

**Emplacement** : `doc/adr/20261030-socle.md:214`, `:301` (secret `db_owner_password` monté dans `store`), `:240` (« connexion dédiée `cadi_owner`, fermée ensuite »), `:412` (`config.Database.OwnerPassword` gardé), `:327` ; spec l. 1012 (« chacun n'est donné qu'au conteneur qui en a besoin »).

**Scénario** : une faille d'exécution dans le store (dépendance vulnérable, décodage) lit `/run/secrets/db_owner_password` ou la mémoire du processus, se connecte en `cadi_owner` et modifie le schéma, supprime la base ou réécrit les rappels. Le secret, utile quelques secondes au démarrage, reste monté et en mémoire pendant toute la vie du conteneur exposé aux trois appelants.

**Correction proposée** (architecte ; PO pour l'arbitrage) :
- option A : un service compose à exécution unique `store-init` (même image, sous-commande `cadi-store init-schema`, `restart: "no"`), seul à recevoir `db_owner_password`, sur le seul réseau `db` ; `store` en dépend par `depends_on: condition: service_completed_successfully` et ne reçoit que `db_app_password`. Le binaire du service base de données crée toujours le schéma, ce qui reste conforme à la spec l. 1010-1012 (à préciser par le PO si besoin) ;
- option B (minimale) : garder la conception, et écrire dans *Impacts sécurité* que la séparation DDL/DML protège contre l'injection SQL et non contre la compromission du store.

**Exigences** : spec l. 1012 ; ASVS V1.2.2, V2.10.1, V2.10.4.

---

## S20 — Mots de passe des rôles dans les traces de PostgreSQL ; rotation — basse

**Gravité** : basse. La lecture par `\set … \`cat …\`` évite l'argument et la variable d'environnement (bien), mais la valeur est envoyée en clair dans `CREATE ROLE … PASSWORD`, que PostgreSQL écrit dans ses traces si l'instruction échoue (`STATEMENT:` du message d'erreur) ou si `log_statement` la couvre. Les traces de `postgres` vont dans `docker logs` ; la spec interdit tout secret dans des traces conservées un an.

**Emplacement** : `doc/adr/20261030-socle.md:234` ; `:68` (CA14, limité à `cadi-store`) ; `:97` (CA31, journalisation bornée de `postgres`) ; `:262` (rotation des jetons seulement).

**Scénarios** :
1. Le script échoue sur `CREATE ROLE` (rôle existant dans une image de test réutilisée, erreur de syntaxe, option ajoutée plus tard) : la trace d'erreur de `postgres` contient l'instruction avec le mot de passe.
2. Le script est écrit avec `set -x`, ou psql lancé avec `-a`/`-e` (`ECHO`) : l'instruction interpolée est écrite sur la sortie du conteneur.
3. Rotation : le script ne s'exécute que sur un volume vide ; changer un fichier de secret ne change pas le mot de passe du rôle, et le store ne se connecte plus. Aucune procédure (`ALTER ROLE`) n'est décrite.

**Correction proposée** (architecte, puis devops et qa) : dans `10-roles.sh`, avant `CREATE ROLE`, `SET log_statement = 'none'` et `SET log_min_error_statement = 'panic'` dans la même session (le superutilisateur le peut) ; ni `set -x`, ni `-a`, ni `-e` ; `ON_ERROR_STOP=1`. Nouveau CA : « Étant donné le compose initialisé, y compris après un échec provoqué du script, `docker compose logs postgres` ne contient aucune valeur des trois fichiers de secrets ». Décrire la rotation des mots de passe de la base (`ALTER ROLE` par le superutilisateur avec les mêmes précautions, puis redémarrage du store), ou écrire qu'elle passe par la recréation de la base avant le lancement.

**Exigences** : spec *Sécurité*, *Conformité* (traces sans secret) ; ASVS V7.1.1, V2.10.4.

---

## S21 — Réseaux et durcissement : CA30 et CA31 incomplets — basse

**Gravité** : basse (développement, sur un poste). Les décisions de D10 sont bonnes ; les CA n'en vérifient qu'une partie.

**Emplacement** : `doc/adr/20261030-socle.md:96` (CA30), `:97` (CA31), `:300`, `:306-307`.

**Constats** :
1. CA30 teste les réseaux de `api` et de `collector`, pas ceux de `notifier` (vers `api:8080`, `collector`, `postgres:5432`). Le notifier parlera à un prestataire d'envoi : il a le même besoin d'isolement que le collector.
2. CA30 ne vérifie pas l'appartenance aux réseaux : seuls `store` et `postgres` sur `db`, `store` absent de `edge`, un seul appelant par réseau `store-*`. Un service ajouté par erreur à `store-api` partagerait le segment de l'API sans échec de test.
3. CA31 n'impose à `postgres` que la journalisation bornée, alors que c'est le conteneur qui détient toutes les données et le mot de passe du superutilisateur. L'image officielle supporte `no-new-privileges`, des limites mémoire/`pids_limit` et `cap_drop: [ALL]` avec un `cap_add` minimal (`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID`), à confirmer par devops sur l'image épinglée.
4. `tmpfs` `/run/cadi` : options non fixées (`noexec`, `nosuid`, `nodev`, taille bornée).

**Correction proposée** (architecte, puis devops) : étendre CA30 au notifier et à l'appartenance aux réseaux (lecture de `docker compose config`) ; étendre CA31 à `postgres` (`no-new-privileges`, limites, `cap_drop`/`cap_add` minimal) et aux options du `tmpfs`.

**Exigences** : ASVS V1.14.1, V14.1.x ; API8:2023 ; spec l. 1012 (« réseaux séparés »).

---

## S22 — Table des autorisations : double source et mutabilité — basse

**Gravité** : basse. Sans effet au lot 0a (une opération ouverte à tous) ; c'est le mécanisme central d'autorisation de tous les lots suivants.

**Emplacement** : `doc/api/store.yaml:32` (« Ouverte à : api, collector, notifier. », texte libre) ; `doc/adr/20261030-socle.md:503` (`var DefaultPermissions`, exportée et modifiable) ; `:63` (CA12 vérifie qu'une opération est classée, pas comment) ; `:268-272` (principes non vérifiables mécaniquement).

**Scénarios** :
1. Un ADR ouvre une opération à l'API dans la table Go et oublie la ligne « Ouverte à » du contrat, ou l'inverse : le contrat, qui fait foi pour le relecteur, ment ; CA12 passe.
2. Une opération sur des données personnelles est ouverte au collector (contraire à D7, `:271`) : seule la relecture humaine le détecte.
3. Tout package qui importe `store` peut écrire `store.DefaultPermissions["op"] = …` à l'exécution (aujourd'hui seul `cmd/cadi-store`, mais rien ne l'interdit).

**Correction proposée** (architecte, puis backend-go et qa) : porter la liste dans le contrat par une extension (`x-cadi-appelants: [api]`, et `x-cadi-donnees-personnelles: true` le cas échéant) ; CA12 compare l'extension à la table et échoue si une opération marquée données personnelles est ouverte au collector. Rendre la table non modifiable (variable non exportée et fonction qui renvoie une copie, ou `Permissions` passée seulement à `NewHandler`).

**Exigences** : ASVS V4.1.3, V1.4.1 ; API5:2023 ; spec l. 1007.

---

## S23 — *Write once* par les droits : portée à préciser — info

`doc/adr/20261030-socle.md:138`, `:239` (« jamais `UPDATE` sur les tables de rappels »). Aucun effet au lot 0a. Pour les ADR suivants :
- la spec rend aussi immuables les produits (l. 287) et les magasins (l. 320), et ne permet sur une marque que `id_marque_cible` (l. 276) : `UPDATE` jamais accordé sur ces tables, ou seulement par colonne (`GRANT UPDATE (id_marque_cible) ON marques`) ;
- les tables de liens d'un rappel (marques, produits, enseignes, magasins, départements) font partie du rappel affiché : pas d'`UPDATE` non plus ; `DELETE` reste nécessaire pour la suppression par l'administrateur (l. 333, 966), à ouvrir à la seule opération d'administration ;
- les fonctions créées par `cadi_owner` sont exécutables par `PUBLIC` par défaut : `ALTER DEFAULT PRIVILEGES FOR ROLE cadi_owner REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` dans `schema.sql`, et aucune fonction `SECURITY DEFINER` sans revue ;
- ne jamais utiliser `ALTER DEFAULT PRIVILEGES … GRANT … TO cadi_app` : les droits restent accordés table par table, comme le prévoit D6.

À écrire dans les principes de D6 (architecte).

## S24 — Cache de santé : attente non annulable — info

`doc/adr/20261030-socle.md:289`. Avec un `sync.Mutex`, les requêtes qui attendent le rafraîchissement ne peuvent pas abandonner quand leur client part : sous afflux, pendant un store lent, chaque requête `/api/sante` garde sa goroutine et sa connexion jusqu'à 2 s. L'effet est borné (`HealthTimeout`, délais du serveur) et ne touche ni le store ni PostgreSQL. Préférer une attente sur canal avec `select` sur le `ctx` de la requête (équivalent de `singleflight.DoChan`), sans changer le contrat ni les CA. Backend-go.

## S25 — Cohérence ADR ↔ spec sur les mots de passe — info

La spec précisée (l. 1007 : « seul, avec PostgreSQL, à détenir les mots de passe de ses rôles » ; l. 1012 : mots de passe des rôles au service base de données et à PostgreSQL, celui du superutilisateur à PostgreSQL seul) est **cohérente** avec D6 (`:234-238`), le tableau de D10 (`:300-301`) et CA30 (`:96`). Restent à mettre à jour dans l'ADR (architecte) :
- F1 (`:28`) cite encore l'ancienne formulation « seul à détenir son mot de passe » ;
- Q1 (`:626`) est résolue par la spec : la retirer ou la marquer tranchée ;
- si l'option A de S19 est retenue, la phrase de la spec reste vraie (le binaire du service base de données reçoit les deux mots de passe, dans deux conteneurs) mais mérite d'être précisée par le PO.

Les jetons du store ne figurent pas dans la liste d'exemples de secrets de la spec (l. 1012) ; la règle « chacun n'est donné qu'au conteneur qui en a besoin » les couvre, et D10/CA30 l'appliquent.

---

## Vérifié sans problème

**Authentification du store**
- Un jeton par service, 256 bits aléatoires, lu par fichier `*_FILE`, jamais en variable d'environnement ni en argument (D5, `:221-223`, `:230`) ; format vérifié au démarrage, jetons identiques refusés par le store (CA13) : aucun jeton partagé possible par configuration.
- Chaque jeton monté dans son service et dans `store` seulement, vérifié par `docker compose config` (CA30) ; les réseaux par appelant (`:301-306`) empêchent en plus qu'un jeton volé soit utilisé depuis un autre conteneur applicatif.
- Comparaison : SHA-256 du jeton présenté puis `crypto/subtle` contre les trois empreintes, toujours toutes (`:258`) : ni fuite par la longueur ni par l'ordre de comparaison ; l'empreinte non salée est suffisante pour un secret de 256 bits.
- Authentification avant tout routage, sur tous les chemins dont `/sante` et les chemins inconnus (CA9) : aucune énumération des opérations sans jeton.
- Refus par défaut : opération absente de la table ou service non listé → `403` (CA10) ; test matriciel service × opération (`:261`) ; table et contrat comparés (CA12) — sous les réserves de S17 et S22.
- `bearerAuth` global dans `store.yaml:22-23`, réponses `401`/`403` déclarées, schéma `Error` fermé (`additionalProperties: false`, énumération).
- Jeton absent des traces : traces d'accès sans en-têtes (CA15), événement `store_acces_refuse` sans valeur (CA9), `config.Secret` masqué partout (CA14), `CADI_STORE_URL` sans identifiants (CA13).
- Client : redirections refusées (`CheckRedirect`, CA29) — le jeton ne peut pas partir vers un autre hôte ; en-tête ajouté par le client lui-même, jamais recopié d'une requête entrante ; `401`/`403` → `ErrStoreFailed`, donc `/api/sante` répond `indisponible` sans détail (CA2).
- *Confused deputy* : l'API n'appelle le store pour un client anonyme que par `/api/sante`, qui ne renvoie qu'un booléen, au plus une fois par 5 s ; le risque résiduel « API compromise = toutes ses opérations pour tous les utilisateurs » est écrit (`:329`) et encadré par les principes de D7 (`:268-272`). Aucune opération ne permet à l'API d'agir avec l'identité du collector ou du notifier.
- `/sante` authentifiée sans casser les `healthcheck` : ceux-ci lisent le fichier local, sans jeton ni port (CA18).

**Rôles PostgreSQL**
- Deux rôles `NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS` ; `cadi_app` sans DDL, `NOINHERIT` (`:235-236`).
- `REVOKE ALL ON DATABASE cadi FROM PUBLIC` puis `CONNECT` aux deux rôles : `cadi_app` n'a ni `CREATE` (schémas) ni `TEMP` sur la base ; `REVOKE CREATE ON SCHEMA public FROM PUBLIC` (redondant mais sans risque sur PostgreSQL ≥ 15, où `public` appartient à `pg_database_owner`, donc à `cadi_owner` propriétaire de la base).
- Objets créés par `cadi_owner`, qui en est propriétaire ; aucun privilège par défaut accordé à `cadi_app` ; droits accordés table par table (`:239`, `schema.sql:560`).
- Superutilisateur : secret distinct, monté dans `postgres` seulement, jamais utilisé par un binaire CADI (`:238`, CA30) ; refus de démarrer si un rôle est superutilisateur, avant toute création ou ouverture de port (CA26, sous la réserve de S18).
- Mots de passe lus dans le script par `\set … \`cat …\``, jamais en argument ni en variable d'environnement (`:234`, sous la réserve de S20).
- `EnsureSchema` : connexion dédiée fermée, verrou consultatif, vérification du propriétaire de la base (refuse `postgres` et toute base étrangère), détection élargie aux vues, fonctions, séquences et schémas non système, `search_path=public`, transaction unique (CA21–CA25, CA28) : S14 traité.

**Santé**
- Aucune requête anonyme n'atteint PostgreSQL : `store.Server` ne détient pas le pool (`:288`, `:513`), seul le moniteur fait `DB.Ping` toutes les 60 s (D9) ; vérifié par CA4 (deuxième partie) et par la structure des types.
- Cache de l'API : un seul appel au store à la fois et au plus un par 5 s et par conteneur, erreurs mises en cache aussi (pas de martèlement d'un store arrêté), contexte détaché borné à 2 s (D9, CA3, CA4) ; S3 et S13 traités.
- `/api/sante` : public, corps fermé `ok`/`indisponible`, `no-store`, aucune cause renvoyée (CA1, CA2) ; conforme à la spec l. 249.
- Fichier de santé : écriture atomique par renommage, `tmpfs` propre à chaque conteneur, aucun volume partagé, dates futures bornées (CA18) ; la sous-commande ne lit aucun secret et n'ouvre aucune connexion.
- `healthcheck` de l'API sur `200` ou `503` : il mesure l'API seule, sans boucle de redémarrage liée aux dépendances (CA19).
- Collector et notifier sans port (CA20) : rien à atteindre sur leurs réseaux.

**Réseaux et durcissement**
- Réseaux `db`, `store-api`, `store-collector`, `store-notifier` en `internal: true` ; `edge` pour la seule publication du port de l'API, sur `127.0.0.1` (D10) : S6 traité.
- CA31 vérifiable par `docker compose config` pour les quatre services CADI : S8 traité (sous la réserve de S21 pour `postgres`).
- Délais cohérents (`CADI_STORE_TIMEOUT` ≤ 8 s < `WriteTimeout` 10 s) : S16 traité.

## Non couvert

- Revue de conception seule : aucun code, aucun outil lancé (`govulncheck`, `gosec`, fuzzing, Trivy sans objet à ce stade).
- Comportement du code généré par oapi-codegen v2 (ordre liaison → décodage → intergiciels stricts, forme du nom d'opération, S17) : établi d'après la v1.6.0 du cache local et la connaissance de l'outil ; à confirmer par backend-go sur la version épinglée.
- Comportement de l'image PostgreSQL officielle (capacités nécessaires, S21 ; traces de l'initialisation, S20) et sémantique de `public` selon la version majeure : à confirmer par devops sur l'image épinglée.
- `doc/api/openapi.yaml`, le lot 0b (frontend) et la CI n'ont pas été relus : hors du périmètre de cette revue ciblée.
