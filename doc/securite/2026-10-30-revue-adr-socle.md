# Revue de conception sécurité — ADR 20261030 « Socle de l'application (lot 0) », version 0.2

- Date : 2026-10-30
- Branche : `feature/socle`
- Objet : `doc/adr/20261030-socle.md` (v0.2, statut « proposé »), `doc/api/openapi.yaml`, `doc/api/store.yaml`
- Type : revue de conception (aucun code n'existe ; aucun outil d'analyse lancé)
- Référentiel : `doc/specifications.md`, chapitres *Sécurité* (l. 203-224), *Conformité* (l. 138-201), *Gestion des risques* (l. 1131-1149), *Gouvernance* (l. 240-249), *Backend* (l. 996-1039) ; OWASP ASVS 4.0.3 niveau 2 (identifiants `Vx.y.z` de la version 4.0.3) ; OWASP API Security Top 10 2023.

## Décisions du PO prises comme données

Non remises en cause ; seules leurs conséquences sont évaluées : `/api/sante` public (`ok`/`indisponible`) ; conteneur `store` seul sur le réseau de PostgreSQL et seul détenteur du mot de passe, appelé en HTTP/JSON (`store.yaml`), une instance ; aucune authentification entre conteneurs au lot 0, jeton par service et liste d'opérations autorisées ensuite ; pas d'outil de migration, schéma créé au démarrage ; secrets par fichiers `*_FILE` ; compose de production reporté.

## Synthèse

Aucun constat critique ni haut : le lot 0 ne manipule aucune donnée, et les choix structurants (store seul détenteur du mot de passe, aucun SQL libre, secrets par fichiers, réseaux `internal`, `/api/sante` sans détail) sont sains. Les constats portent sur ce que le socle fige pour tous les lots suivants : rôle PostgreSQL, conventions d'authentification du store, gestion des erreurs générées, masquage des secrets, coût de la santé publique.

| # | Gravité | Sujet |
|---|---|---|
| S1 | moyenne | Le store se connectera vraisemblablement en superutilisateur PostgreSQL ; la séparation DDL/DML est reportée alors qu'elle dépend de D6 |
| S2 | moyenne | Authentification du store : déclencheur mal choisi (« lot de la collecte ») et conventions à fixer dès le socle |
| S3 | moyenne | `/api/sante` public sans mise en cache : chaque requête anonyme traverse API → store → pool PostgreSQL |
| S4 | moyenne | Gestionnaires d'erreurs par défaut du serveur strict oapi-codegen : texte de l'erreur Go renvoyé au client |
| S5 | moyenne | Masquage du mot de passe porté par `config.Database` seulement : contourné par `json`, slog imbriqué, `%#v` |
| S6 | basse | Réseau `store` unique partagé par api, collector et notifier |
| S7 | basse | Contrat public : pas de règle « refus par défaut » vérifiable |
| S8 | basse | Durcissement des conteneurs et rotation des traces sans critère d'acceptation |
| S9 | basse | Client du store et serveurs : redirections, tailles de corps, connexions non bornées |
| S10 | basse | Traces d'accès non définies par le socle |
| S11 | basse | CI : outils non épinglés, `persist-credentials`, image PostgreSQL des tests, scan d'image absent |
| S12 | info | Pas de TLS interne (`CADI_DB_SSLMODE=disable`) : écart ASVS V9.2.2 à déclarer comme accepté |
| S13 | info | `/api/sante` : la cause d'indisponibilité se déduit du temps de réponse |
| S14 | info | Détection de « base inconnue » limitée aux tables de `public` ; démarrages concurrents |
| S15 | info | Points reportés à cadrer : envoi des emails de connexion, SSRF depuis le collector |
| S16 | info | Délais incohérents : `CADI_STORE_TIMEOUT` jusqu'à 60 s, `WriteTimeout` du store à 10 s |

---

## S1 — Rôle PostgreSQL : superutilisateur probable, séparation DDL/DML reportée à tort — moyenne

**Gravité** : moyenne. Aucune requête métier au lot 0, donc rien d'exploitable aujourd'hui ; mais le rôle choisi par le socle est celui qu'utiliseront tous les lots, et un superutilisateur transforme toute injection SQL future dans le store en exécution de commandes dans le conteneur PostgreSQL.

**Emplacement** :
- `doc/adr/20261030-socle.md:210` — « Un seul rôle PostgreSQL (propriétaire) en lot 0 […] séparation des rôles DDL/DML sont [R] » ;
- `doc/adr/20261030-socle.md:245-246, 252` — `postgres` reçoit `POSTGRES_PASSWORD_FILE`, le même secret `db_password` est monté dans `postgres` et `store` ;
- `doc/adr/20261030-socle.md:508` — « rôles PostgreSQL séparés » reportés au lot du déploiement de production.

**Scénario** : avec l'image officielle, la manière la plus directe d'écrire le compose est `POSTGRES_USER=cadi`, `POSTGRES_DB=cadi`, `POSTGRES_PASSWORD_FILE=/run/secrets/db_password`. L'image crée alors `cadi` **avec les droits de superutilisateur**, et le store s'y connecte (`CADI_DB_USER=cadi`, même secret). Une injection SQL dans une opération future du store (requête construite par concaténation d'un nom de colonne de tri, par exemple) donne `COPY … TO PROGRAM` : exécution de commandes dans le conteneur `postgres`, lecture de tous les fichiers de données, désactivation des contraintes *write once*. Sans aller jusque-là, un rôle propriétaire peut `DROP`/`TRUNCATE`/`ALTER` à l'exécution, ce que la logique « le schéma n'est créé qu'au démarrage » (D6) n'exige pas.

Le report au « lot du déploiement de production » est mal placé : la séparation dépend de D6 (qui exécute le DDL, avec quels identifiants), donc de la configuration (`CADI_DB_*`) et de `EnsureSchema`, que le socle fige. La poser plus tard changera `config.Database`, `store.Open`, `EnsureSchema`, `schema.sql` et le compose.

**Correction proposée** (architecte, puis devops et backend-go) :
1. Deux rôles dès le lot 0, aucun superutilisateur côté application :
   - `cadi_owner` (`NOSUPERUSER NOCREATEDB NOCREATEROLE`), propriétaire du schéma, utilisé **uniquement** par `EnsureSchema` sur une connexion dédiée fermée ensuite ;
   - `cadi_app` (`NOSUPERUSER`, aucun droit DDL), utilisé par le pool ; `schema.sql` accorde les droits par table (`GRANT SELECT, INSERT … TO cadi_app`). Bénéfice direct pour la spec : le *write once* des rappels peut être garanti en n'accordant pas `UPDATE` sur la table des rappels.
   - Variables : `CADI_DB_OWNER_USER`, `CADI_DB_OWNER_PASSWORD_FILE`, `CADI_DB_USER`, `CADI_DB_PASSWORD_FILE`.
2. Si le PO garde un seul rôle au lot 0 : au minimum, rôle **non superutilisateur**, créé par un script d'initialisation du conteneur `postgres` ; secret du superutilisateur `postgres` (`db_admin_password`) distinct de celui du store et jamais monté dans `store`.
3. Nouveau CA : « Étant donné un rôle superutilisateur (`rolsuper`) dans `CADI_DB_USER`, quand `cadi-store` démarre, alors il sort avec le code `2` sans ouvrir de port » (vérification `SELECT rolsuper FROM pg_roles WHERE rolname = current_user` dans `EnsureSchema`). Étendre CA21 : `docker compose config` montre que le secret du superutilisateur n'est monté que dans `postgres`.

**Exigences** : spec *Backend > Architecture* l. 1007 (« seul autorisé à lire et écrire la DB »), *Workflow > Write once* (agent `securite`) ; ASVS V1.2.1 (comptes de faible privilège par composant), V1.2.2 (moindre privilège entre composants), V2.10.2 (pas de compte par défaut / privilégié pour un service), V4.1.3 (moindre privilège).

---

## S2 — Authentification entre conteneurs : déclencheur et conventions à fixer dans le socle — moyenne

**Gravité** : moyenne. Le plan du PO (jeton par service + liste d'opérations autorisées) est le bon mécanisme et suffit contre l'abus d'opérations réservées. Deux lacunes : le déclencheur, et l'absence de conventions dans le socle, qui imposera une reprise de signatures et du contrat.

**Emplacement** :
- `doc/adr/20261030-socle.md:222` — « au lot de la collecte (première opération réservée) » ; menace limitée au « conteneur API compromis » ;
- `doc/adr/20261030-socle.md:276, 507, 516` ;
- `doc/adr/20261030-socle.md:320, 362` — `config.StoreClient{URL, Timeout}`, `storeclient.New(baseURL, timeout)` : aucun emplacement pour un jeton ;
- `doc/adr/20261030-socle.md:396` — `store.NewHandler(s)` : aucun emplacement pour un intergiciel d'autorisation ;
- `doc/api/store.yaml:1-51` — ni `securitySchemes`, ni réponses `401`/`403`, ni convention.

**Scénarios** :
1. *Déclencheur.* Le lot de la connexion est annoncé avant celui de la collecte (`:509`). Il ajoutera au store des opérations sur les comptes, sessions et jetons (données personnelles). Sans authentification, tout conteneur du réseau `store` peut alors les appeler. Or l'API n'est pas le seul composant exposé : **le collector analyse des données externes non fiables** (fiches RappelConso, textes libres) et aura un accès Internet sortant ; une faille dans son analyse lui donnerait, sans liste d'opérations, la lecture de tous les comptes et emails. Le notifier, lui, parle à un prestataire d'envoi.
2. *Sante et `healthcheck`.* CA9 impose que la sous-commande `healthcheck` ne lise aucun secret ; si `/sante` du store passe un jour sous jeton, le `healthcheck` compose du store casse, ou on lui donne le jeton. Le choix doit être fait maintenant.
3. *Risque résiduel à écrire.* Même avec la liste, une API compromise garde toutes les opérations autorisées à l'API, pour **tous** les utilisateurs : le store ne connaît pas l'utilisateur final. Le mécanisme limite la fonction (API5:2023), pas l'objet (API1:2023). C'est acceptable sur un seul serveur mais doit être explicite, et orienter la conception des opérations.

**Correction proposée** (architecte) — à écrire dans D7 dès la v0.3, sans implémentation au lot 0 :
1. **Déclencheur** : l'authentification et la liste d'opérations sont livrées avec **la première opération ajoutée à `store.yaml` après `/sante`**, quel que soit le lot (et non « le lot de la collecte »).
2. **Conventions** :
   - `store.yaml` : schéma `bearerAuth` (`type: http`, `scheme: bearer`), exigé globalement ; `/sante` explicitement `security: []` (reste public pour le `healthcheck`, ne renvoie rien) ; réponses `401` (jeton absent ou inconnu) et `403` (opération non autorisée à ce service), sans corps détaillé.
   - Jeton : 256 bits aléatoires, un fichier de secret par service (`store_token_api`, `store_token_collector`, `store_token_notifier`), monté dans le service et dans `store` seulement ; variable `CADI_STORE_TOKEN_FILE` côté clients ; comparaison en temps constant ; rotation par redémarrage ; jamais dans les traces (en-tête `Authorization` exclu des traces d'accès, voir S10).
   - Serveur : intergiciel strict (`StrictMiddlewareFunc` reçoit l'`operationID`) qui identifie l'appelant par son jeton et applique une table `operationID → services autorisés`, **refus par défaut** : une opération absente de la table est refusée à tous. Un refus est tracé (service appelant, `operationID`) comme événement de sécurité.
   - Test de conformité : chaque `operationId` de `store.yaml` figure dans la table (une opération ajoutée sans classement fait échouer la CI) ; test matriciel service × opération.
   - Signatures : `storeclient.New(baseURL string, timeout time.Duration, token string)` et `config.StoreClient{URL, Timeout, Token}` dès le lot 0 (jeton vide accepté tant que le store n'exige rien), `store.NewHandler(s *Server, authz Authorizer)` ; ainsi le lot suivant ajoute une implémentation, pas une refonte.
3. **Principes de conception des opérations** (à écrire dans D7) : les opérations de l'API portent sur un `id_user` explicite et ne renvoient que les données de cet utilisateur ; aucune opération de lecture en masse des emails n'est autorisée à l'API (seul le notifier lit les destinataires d'un rappel) ; le collector n'a aucune opération sur les données personnelles.
4. Réponse à Q1 : l'option « aucune authentification au lot 0 » est acceptable (seul `/sante`, sans donnée), **à condition** des points 1 et 2. Avec S6 (réseaux par appelant), le jeton devient une défense en profondeur et non la seule barrière.

**Exigences** : spec *Backend > Architecture* l. 1007, *Interfaces* l. 1035 ; ASVS V1.2.2 (communications entre composants authentifiées, moindre privilège), V2.10.1 (secrets inter-services), V4.1.3, V4.1.5 (échec sûr / refus par défaut), V7.1.3 (événements de sécurité tracés) ; API5:2023 *Broken Function Level Authorization*.

---

## S3 — `/api/sante` public : amplification jusqu'au pool PostgreSQL — moyenne

**Gravité** : moyenne. Accessible sans session ni limitation de débit (reportée au lot de la connexion), chaque requête consomme une connexion du pool partagé du store (10 par défaut). Le point de santé survit au lot 0 : il partagera ce pool avec toutes les opérations métier.

**Emplacement** :
- `doc/adr/20261030-socle.md:260-262` (D11 : chaîne complète à chaque requête) ;
- `doc/adr/20261030-socle.md:189` (`CADI_DB_MAX_CONNS` = 10), `:221` (`MaxIdleConnsPerHost` 10, aucune borne `MaxConnsPerHost`) ;
- `doc/adr/20261030-socle.md:277` (limitation du débit [R]) ;
- `doc/api/openapi.yaml:21-26`.

**Scénario** : un client anonyme envoie quelques centaines de `GET /api/sante` par seconde (ou simplement le trafic d'un pic de consultation après un rappel médiatisé, puisque la page d'accueil appelle `/api/sante` à chaque ouverture, CA17-CA18). Chaque requête ouvre ou réutilise une connexion API → store, et chaque `Ping` du store prend une connexion du pool : les opérations métier attendent une connexion libre, puis dépassent leur délai. Avec plusieurs conteneurs API, le facteur se multiplie. Le PostgreSQL lui-même tient, mais le service devient « indisponible » pour tous, par une route publique, sans authentification.

**Correction proposée** (architecte, puis backend-go) :
1. `logic.Service.CheckHealth` met en commun les vérifications concurrentes (`singleflight`) et garde le résultat pendant un court intervalle (ex : `HealthCacheTTL` = 2 s) : au plus un appel au store par intervalle et par conteneur API, quel que soit le trafic. CA3 et CA15 restent vrais à l'intervalle près.
2. Optionnel côté store : même mise en commun du `Ping`.
3. Borne `MaxConnsPerHost` sur le transport du client du store (ex : égale à `CADI_DB_MAX_CONNS`), pour que la file d'attente se forme côté API, bornée par le délai de la requête.
4. Nouveau CA : « Étant donné 100 requêtes `GET /api/sante` concurrentes, quand elles sont servies, alors le store reçoit au plus 1 appel `GET /sante` par intervalle `HealthCacheTTL` » (test unitaire `logic` avec store simulé comptant les appels).

La limitation du débit peut alors rester au lot de la connexion.

**Exigences** : spec *Gestion des risques* l. 1144 et 1147 (backend indisponible, forte affluence) ; ASVS V11.1.4 (contrôles anti-automatisation) ; API4:2023 *Unrestricted Resource Consumption*.

---

## S4 — Erreurs : gestionnaires par défaut du serveur strict oapi-codegen — moyenne

**Gravité** : moyenne. Le gestionnaire `GetHealth` prévu ne renvoie pas d'erreur Go, donc le lot 0 n'est pas touché ; mais le comportement par défaut du code généré s'appliquera à toutes les routes de tous les lots (API publique et store), et CA2/CA4 ne le testent pas.

**Emplacement** :
- `doc/adr/20261030-socle.md:215, 228` (mode `strict-server`), `:396, 409` (`NewHandler`) ;
- `doc/adr/20261030-socle.md:216` (« erreur inattendue → `500` sans détail ») : intention juste, mécanisme non décrit ;
- `doc/api/store.yaml:14` ; `doc/api/openapi.yaml` (aucune réponse `500` déclarée).

**Scénario** : dans le code généré par oapi-codegen v2 pour `strict-server` (à confirmer par backend-go sur la version épinglée), `NewStrictHandler` installe par défaut `ResponseErrorHandlerFunc` = `http.Error(w, err.Error(), 500)` et `RequestErrorHandlerFunc` = `http.Error(w, err.Error(), 400)`. Un futur gestionnaire qui renvoie `fmt.Errorf("lecture du rappel: %w", err)` enveloppant une erreur du client du store renvoie au navigateur, en `text/plain`, quelque chose comme `… Get "http://store:8090/…": dial tcp 172.20.0.3:8090: connect: connection refused` : nom d'hôte, adresse et port internes, topologie. Côté store, une erreur pgx renvoyée par erreur expose noms de tables et de contraintes. Une `panic` dans un gestionnaire coupe la connexion sans trace exploitable.

**Correction proposée** (architecte, puis backend-go) :
1. D4 : `api.NewHandler` et `store.NewHandler` utilisent `NewStrictHandlerWithOptions` avec des gestionnaires de `platform` : `400` → `{"code":"requete_invalide"}`, `500` → `{"code":"erreur_interne"}`, `Content-Type: application/json`, `Cache-Control: no-store`, cause tracée avec un identifiant de requête, jamais renvoyée. Même chose pour l'`ErrorHandlerFunc` du wrapper non strict (liaison des paramètres).
2. Intergiciel de récupération des `panic` dans `platform` (trace + `500` générique).
3. Schéma d'erreur commun déclaré dans `openapi.yaml` et `store.yaml` dès le lot 0 (réponse `default`), plutôt qu'« introduit par la première opération qui en a besoin ».
4. Nouveau CA : « Étant donné un gestionnaire qui renvoie une erreur contenant `store:8090`, ou qui panique, quand un client appelle la route, alors la réponse est `500 {"code":"erreur_interne"}` et ne contient ni le texte de l'erreur ni un nom d'hôte » (API et store).

**Exigences** : agent `securite` (*API REST*, réponses conformes au contrat) ; ASVS V7.4.1 (message d'erreur générique), V14.3.2 ; API8:2023 *Security Misconfiguration*.

---

## S5 — Masquage du mot de passe par le type conteneur, contournable — moyenne

**Gravité** : moyenne. La spec interdit les secrets dans les traces, conservées un an (l. 172, 192, 219). Le masquage prévu ne protège que le cas où `config.Database` est journalisé directement ; la ligne de trace la plus naturelle au démarrage le contourne. La liste des secrets va grandir (email de l'administrateur, clé du prestataire d'envoi, jetons du store).

**Emplacement** : `doc/adr/20261030-socle.md:311-318` (`Password string` champ exporté, `String`/`LogValue` sur `Database`), `:322` (`config.Store` contient `DB Database`), `:275`.

**Scénario** : `log.Info("configuration chargée", "config", cfg)` avec `cfg` de type `config.Store`. `slog` ne résout `LogValuer` que pour la valeur de premier niveau ; `config.Store` n'en implémente pas, et le `JSONHandler` sérialise la structure par `encoding/json`, qui ignore `String` et `LogValue` : le champ `DB.Password` est écrit en clair. De même `fmt.Sprintf("%#v", cfg.DB)` (`GoString`) ou toute sérialisation JSON de la configuration pour un diagnostic.

**Correction proposée** (architecte, puis backend-go) : un type `config.Secret` (chaîne opaque) qui porte lui-même le masquage : `String`, `GoString`, `LogValue`, `MarshalJSON`, `MarshalText` renvoient `"***"` ; accès à la valeur par une méthode explicite (`Reveal()`), utilisée seulement au point d'usage (configuration pgx, comparaison de jeton). `Database.Password` devient `config.Secret`. Étendre CA6 : une trace JSON de `config.Store` entier, `%v`, `%+v`, `%#v` et `json.Marshal` ne contiennent pas la valeur du fichier de secret. Construire la configuration pgx par champs (`pgxpool.Config`, `ConnConfig.Password`), jamais par une chaîne de connexion formatée.

**Exigences** : spec *Conformité* l. 172, 192 ; *Sécurité* l. 219 ; ASVS V7.1.1, V7.1.2 (pas de secret ni donnée sensible dans les traces), V2.10.4.

---

## S6 — Réseau `store` partagé par les trois clients — basse

**Gravité** : basse. Au lot 0, il n'y a rien à atteindre (points de santé sans donnée). Le coût de la correction est nul (compose seul) et elle rend S2 défensif plutôt qu'unique.

**Emplacement** : `doc/adr/20261030-socle.md:246-249, 252` (réseau `store` pour `store`, `api`, `collector`, `notifier`) ; spec *Déploiement* l. 1012 (« réseaux séparés »).

**Scénario** : l'API compromise est sur le même segment que `collector:8081` et `notifier:8081`, et le sera de tout ce que ces services exposeront plus tard (déclenchement d'une collecte, envoi d'email si le notifier reçoit des demandes, voir S15). Réciproquement, un collector compromis par une fiche malveillante atteint l'API sans passer par le frontal.

**Correction proposée** (architecte, puis devops) : un réseau `internal: true` par appelant : `store-api` (`store`, `api`), `store-collector` (`store`, `collector`), `store-notifier` (`store`, `notifier`). Étendre CA21 : depuis un conteneur attaché aux seuls réseaux de `api`, les connexions vers `collector:8081` et `notifier:8081` échouent ; aucun service hors `api` et `mailpit` ne publie de port sur l'hôte. Pour la production (lot reporté) : l'API sur un réseau interne joint seulement par le serveur web frontal, sans accès Internet sortant (le réseau `edge` non `internal` du développement lui en donne un).

**Exigences** : ASVS V1.14.1 (séparation des composants de confiance différente), V1.2.2.

---

## S7 — Contrat public : pas de règle de refus par défaut vérifiable — basse

**Gravité** : basse au lot 0 (une seule route, publique par décision du PO). Deviendrait haute si un lot exposant des données communes était livré avant celui de la connexion ou oubliait la déclaration.

**Emplacement** : `doc/api/openapi.yaml:13` (« une opération publique le déclare par `security: []` ») ; aucun `security` global ni `securitySchemes` ; `doc/adr/20261030-socle.md:227`.

**Scénario** : en OpenAPI, une opération sans `security` hérite du `security` global ; sans global, elle est publique. La convention « déclarer `security: []` pour être public » n'a donc aucun effet tant que le schéma par cookie n'existe pas : une route de rappels ajoutée sans rien déclarer serait publique, et rien ne le détecterait. La spec (*Gouvernance* l. 246-249) n'autorise sans connexion que la connexion, le désabonnement par jeton et `/api/sante`.

**Correction proposée** (architecte) : écrire dans D8 la règle « refus par défaut » et sa vérification : (a) dès le lot de la connexion, `security` global obligatoire ; (b) dès le lot 0, un test de conformité (unitaire `api`, lecture du contrat embarqué) qui compare la liste des opérations publiques (`security: []`) à une liste fermée (`getHealth`, puis connexion et désabonnement) et échoue sur toute autre ; (c) côté Go, l'intergiciel de session du lot de la connexion refuse toute `operationID` absente de la liste publique. Tant que le lot de la connexion n'est pas livré, aucune opération non publique n'est ajoutée au contrat (à écrire dans *Points reportés*).

**Exigences** : spec *Gouvernance* l. 246-249, *Interfaces* l. 1037 ; ASVS V4.1.5, V1.4.1 ; agent `securite` (*Accès*).

---

## S8 — Durcissement des conteneurs et rotation des traces sans critère d'acceptation — basse

**Gravité** : basse (développement, sur un poste). Le durcissement est décidé (« durcissement de l'agent devops dès le développement », `:252`) mais ne se vérifie qu'en conformité ; CA21 ne couvre que réseau et secret.

**Emplacement** : `doc/adr/20261030-socle.md:169, 252` ; `.claude/agents/devops.md` (*Docker*, *Docker compose*).

**Scénario** : un oubli de `read_only`, `cap_drop` ou d'utilisateur non root sur un service ne fait échouer aucun test. Sans `logging` borné, le pilote `json-file` par défaut grossit sans limite : un client qui fait tracer des erreurs en boucle remplit le disque, ce qui arrête PostgreSQL.

**Correction proposée** (architecte, puis devops) : CA22 vérifiable par `docker compose config` : pour `store`, `api`, `collector`, `notifier` : `read_only: true`, `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, utilisateur non root, limites mémoire/CPU et `pids_limit`, `logging` borné (`max-size`, `max-file`) ; `postgres` et `store` sans `ports`. Image `cadi-backend` : finale distroless `nonroot`, épinglée par digest (déjà dans D4 et l'agent devops). Vérifier que les fichiers de `deploy/secrets/dev/` sont lisibles par l'UID non root du conteneur sans être rendus lisibles par tous sur l'hôte (les options `uid`/`mode` des secrets fichiers ne sont pas appliquées par compose hors Swarm : à vérifier par devops).

**Exigences** : agent devops ; ASVS V14.1.x (configuration sûre du déploiement), V1.2.1 ; API8:2023.

---

## S9 — Client du store et serveurs HTTP : redirections, tailles, connexions — basse

**Gravité** : basse. Le store est un composant interne de confiance ; il s'agit de défense en profondeur bon marché, à figer dans `platform`/`storeclient` qui sont communs à tous les lots.

**Emplacement** : `doc/adr/20261030-socle.md:175` (`MaxHeaderBytes` seul, aucune limite de corps), `:221` (transport), `:362`.

**Scénario** : (a) le client HTTP par défaut suit les redirections : une valeur erronée de `CADI_STORE_URL` ou un intermédiaire inattendu ferait porter les appels (et demain le jeton, S2 : Go retire `Authorization` vers un autre hôte, pas vers le même hôte sur un autre chemin) ailleurs que prévu ; (b) aucune limite de taille de réponse côté client ni de corps de requête côté serveurs : un corps JSON de plusieurs centaines de Mo envoyé à une future route `POST` de l'API est décodé en mémoire.

**Correction proposée** (architecte, puis backend-go) : `storeclient` : `CheckRedirect` qui refuse toute redirection, lecture des réponses bornée (ex : 1 Mio au lot 0, ajustable par opération), `MaxConnsPerHost` (S3). `platform.NewHTTPServer` : `http.MaxBytesHandler` (ex : 64 Kio par défaut pour l'API, ajustable par route ; `413` générique). Les URL du store ne sont construites que par le client généré (échappement des paramètres de chemin), jamais à la main. CA : réponse du store de 2 Mio → `ErrStoreFailed` ; redirection `302` → `ErrStoreFailed` ; corps de requête au-delà de la limite → `413`.

**Exigences** : agent `securite` (*API REST* : taille des corps bornée) ; ASVS V12.1.1 (limite de taille), V12.6.1 ; API4:2023.

---

## S10 — Traces d'accès non définies par le socle — basse

**Gravité** : basse (aucune donnée personnelle au lot 0). Le socle définit `platform` et les traces ; s'il ne fixe pas le format des traces d'accès, chaque lot l'inventera, alors que la spec impose une conservation d'un an, sans email.

**Emplacement** : `doc/adr/20261030-socle.md:176` (règles générales seulement) ; spec *Conformité* l. 172, 192 ; *Purge* l. 992.

**Scénario** : un intergiciel de trace ajouté plus tard journalise l'URL brute (`r.URL.String()`) : le jour où un jeton ou un email passe en paramètre de requête, il est conservé un an ; ou il journalise les en-têtes (`Cookie`, `Authorization` du store, S2).

**Correction proposée** (architecte) : définir dans D4 l'intergiciel de trace d'accès de `platform` : horodatage, identifiant de requête, méthode, **`operationID` ou gabarit de route** (jamais le chemin brut ni la chaîne de requête), code, durée ; jamais de corps ni d'en-tête ; l'adresse IP (donnée personnelle) n'y figure que si la décision est prise, avec sa durée de conservation. Préciser où sont conservées les traces un an (reporté au compose de production, mais à nommer dans *Points reportés*). CA : une requête `GET /api/sante?email=x@example.org` avec un cookie produit une trace qui ne contient ni `x@example.org` ni la valeur du cookie.

**Exigences** : spec *Conformité* l. 172, 192 ; agent `securite` (*RGPD* : pas de données personnelles dans les traces) ; ASVS V7.1.1, V7.1.2, V7.3.1.

---

## S11 — CI : épinglage et permissions — basse

**Gravité** : basse. Les choix principaux sont bons (`contents: read`, actions épinglées par SHA, aucun `push` d'image). Il reste des vecteurs de chaîne d'approvisionnement non couverts.

**Emplacement** : `doc/adr/20261030-socle.md:255` ; `.claude/agents/devops.md` (*CI*).

**Constats** :
1. `govulncheck`, `golangci-lint`, `gosec` : mode d'installation non précisé ; `go install …@latest` n'est pas épinglé. Les épingler comme `oapi-codegen` (directive `tool` de `go.mod`) ou par version et somme de contrôle de l'action.
2. `actions/checkout` avec `persist-credentials: false` (le jeton n'a pas à rester dans `.git/config` pendant `npm ci` et les tests).
3. `npm ci` exécute les scripts d'installation des dépendances avec le dépôt et le jeton accessibles ; évaluer `npm ci --ignore-scripts` (à vérifier par frontend/devops : esbuild).
4. Image PostgreSQL des tests d'intégration (testcontainers) : épinglée par digest, la même que le compose.
5. Contradiction : l'agent devops prévoit un scan d'image (Trivy) et un fuzzing court à chaque PR ; D10 ne les cite pas et `:508` reporte le scan d'image. À trancher explicitement (le scan de `cadi-backend` est possible dès le lot 0 ; le fuzzing attend des cibles).
6. Jamais de déclencheur `pull_request_target` ; `permissions` déclarées au niveau du workflow, chaque job sans droit supplémentaire.

**Correction proposée** : architecte (D10), puis devops.

**Exigences** : spec *Gestion des risques* l. 1148 ; ASVS V14.2.1, V14.2.4, V10.3.2 ; agent devops (*CI*).

---

## S12 — Pas de TLS interne : écart ASVS à déclarer — info

`doc/adr/20261030-socle.md:188` (`CADI_DB_SSLMODE` = `disable`), `:222` (pas de TLS entre conteneurs [R]). ASVS 4.0.3 V9.2.2 (niveau 2) demande le chiffrement des connexions internes, base comprise. Sur un seul serveur, sur des ponts Docker qui ne quittent pas l'hôte, le gain est faible et le report est raisonnable ; l'ADR doit le nommer comme **écart accepté à V9.2.2**, avec la condition de réexamen déjà écrite (`:512`, plusieurs serveurs). Correction : une phrase dans *Impacts sécurité* (architecte).

## S13 — `/api/sante` : cause déduite du temps de réponse — info

`doc/adr/20261030-socle.md:48, 221, 262`. Le corps ne dit rien (bien), mais le temps distingue les causes : `503` immédiat (store arrêté), après ~1 s (PostgreSQL lent, `PingTimeout`), après ~2 s (store lent). Information de faible valeur pour un attaquant ; elle disparaît avec la mise en cache de S3. Aucune autre action.

## S14 — Détection de « base inconnue » et démarrages concurrents — info

`doc/adr/20261030-socle.md:205-208`. (a) Seules les tables de `public` sont comptées : une base contenant d'autres schémas, des vues, fonctions ou extensions, ou la base de maintenance `postgres` (vide de tables) par erreur de `CADI_DB_NAME`, est vue comme vide et reçoit le schéma. Compter aussi les autres objets de `public` et tout schéma non système, ou exiger que la base porte le nom attendu. (b) Deux `cadi-store` simultanés sur une base vide (mise à jour mal ordonnée) : le second échoue sur une table existante et redémarre, sans dommage ; un `pg_advisory_xact_lock` en tête de `EnsureSchema` rend le cas explicite. (c) Fixer `search_path` dans la configuration de connexion (`public` seul). Architecte puis backend-go ; pas de CA nécessaire au-delà de CA12.

## S15 — Points reportés à cadrer avec la sécurité — info

1. **Envoi des emails de connexion** (`:511`). Si l'API demande au notifier d'envoyer un email à une adresse quelconque, le notifier expose une primitive « envoyer un email à X » sur un réseau interne ; une API compromise contourne alors la limite « 3 demandes par heure et par adresse » appliquée dans la logique de l'API (spec l. 214) et utilise la réputation d'envoi de CADI. L'ADR de la connexion devra appliquer la limite là où l'email part (store, par une opération qui compte et refuse, ou notifier), et faire relire ce point par `securite`.
2. **SSRF depuis le collector** (`:510`). Le collector est sur le réseau du store et lira des fiches contenant des URL (pièces jointes, images). L'ADR de la collecte devra imposer un client HTTP distinct de `storeclient`, limité aux hôtes de RappelConso (liste fermée), refusant les adresses privées et les redirections hors liste, avec tailles et délais bornés.

## S16 — Délais incohérents — info

`doc/adr/20261030-socle.md:175, 193`. `CADI_STORE_TIMEOUT` accepte jusqu'à 60 s, mais le `WriteTimeout` du serveur du store est de 10 s : une opération de plus de 10 s est coupée côté store et vue comme `ErrOutcomeUnknown` côté client. Borner `CADI_STORE_TIMEOUT` à une valeur inférieure au `WriteTimeout` du store, ou documenter le couplage. Architecte.

---

## Vérifié sans problème

- **Mot de passe de la base** : lu seulement depuis un fichier (`*_FILE`), donné à `store` (et `postgres`, indispensable à l'initialisation), jamais aux autres services ; jamais en variable d'environnement, donc absent de `docker inspect` (D5, D10, CA21).
- **Frontière PostgreSQL** : `postgres` seul sur `db` avec `store`, réseau `internal`, aucun port publié ; CA21 le vérifie par une connexion TCP réelle et non par la seule lecture du compose.
- **Aucun SQL libre** dans l'interface du store ; une opération = un appel = une transaction (D3, D7, `store.yaml:13`) ; `pgx` importé par le seul `internal/store` (depguard).
- **L'API n'appelle pas le store directement** : `api` ne peut importer ni `store*` ni pgx ; seul `cmd/cadi-api` câble `storeclient` derrière `logic.Store` (D3).
- **`CADI_STORE_URL`** : `http`/`https` seulement, sans identifiants, chemin ni requête (D5, CA5) : pas de fuite d'identifiants dans les traces, pas de schéma inattendu.
- **Configuration invalide** : arrêt avant toute ouverture de port ou connexion ; la trace nomme la variable, jamais sa valeur (CA5).
- **`/api/sante`** : `security: []` explicite, corps fermé (`additionalProperties: false`, énumération), `Cache-Control: no-store` sur `200` et `503`, aucun nom d'hôte ni message (CA1, CA2, `openapi.yaml:26-63`) ; conforme à *Gouvernance* l. 249 (aucune donnée). `405`/`404` sans détail interne (CA4).
- **Store absent au démarrage de l'API** : pas de connexion au démarrage, pas d'état bloqué (CA3).
- **Le store n'ouvre son port qu'après la vérification du schéma** (O3-A, CA13, CA14) ; il ne touche jamais une base non vide inconnue (CA12, sous la réserve de S14) ; création du schéma en une transaction (CA14).
- **Erreurs du client du store** classées sans jamais renvoyer `nil` ni une valeur vide comme résultat (CA16) ; idempotence imposée aux écritures pour le rejeu après `ErrOutcomeUnknown` (D7).
- **Serveur HTTP commun** : `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` fixés (protection *slowloris* sans CA dédié, choix du PO accepté).
- **Sous-commande `healthcheck`** : ne lit aucun secret, permet une image distroless sans shell (D4, CA9).
- **Image** : multi-étapes, distroless, non root (D4).
- **Secrets de développement** : aléatoires, jamais écrasés, `deploy/secrets/` ignoré par git (D10).
- **Ports hôte de développement** : publiés sur `127.0.0.1` seulement (`api`, `mailpit`).
- **Frontend** : aucune ressource tierce, aucun script ni style en ligne dans `index.html` ; client généré ; `fetchHealth` ne rejette jamais et traite tout corps inattendu comme indisponible (D9) ; `package-lock.json` committé et `npm audit` en CI (spec l. 1148, 1149).
- **CI** : `contents: read`, actions épinglées par SHA, aucun envoi d'image (D10).
- **Code généré** : committé, régénéré et comparé par la CI (D8) : le contrat ne peut pas diverger du serveur sans échec.
- **Données personnelles** : aucune au lot 0 ; seule table `schema_version` (D6).
- **Reports légitimes** (sans risque au lot 0, aucune donnée) : CSP, HSTS, `X-Content-Type-Options` au serveur web de production ; TLS 1.3 / X25519MLKEM768 du frontal (spec l. 169) ; limitation du débit (sous réserve de S3) ; CSRF et cookie de session ; sauvegardes chiffrées.
- **Raisonnement de D7 sur le jeton partagé** : exact — un jeton commun à api, collector et notifier n'apporterait rien contre une API compromise.

## Non couvert

- Revue de conception seule : aucun code, aucun outil lancé (`govulncheck`, `gosec`, `npm audit`, Trivy, fuzzing sans objet à ce stade).
- Le comportement par défaut des gestionnaires d'erreurs d'oapi-codegen (S4) est cité d'après la connaissance de l'outil ; le module n'est pas dans le cache local : à confirmer par backend-go sur la version épinglée.
- Le comportement de l'image PostgreSQL officielle pour `POSTGRES_USER` (S1) est cité d'après sa documentation ; à confirmer par devops sur l'image épinglée.
- Le compose de production, le serveur web frontal, les sauvegardes et la purge ne sont pas encore conçus : hors de cette revue.
