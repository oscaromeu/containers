# allquiet-sync

Espejo temporal de [flanksio/containers#440](https://github.com/flanksio/containers/pull/440)
para construir la imagen arm64 que corre el PoC en el homelab. La fuente de
verdad del código es flnx; al acabar el PoC esta copia se borra.

Sync one-shot AllQuiet → `e2e.incidents`: lee los veredictos de triaje
(Affects = downtime, Archive sin affects = falso positivo), ancla cada ventana
al episodio real de `e2e.runs` y reconcilia por diff idempotente. Secretos por
env (`ALLQUIET_API_KEY`, `CLICKHOUSE_PASSWORD`), comportamiento por flags,
`--dry-run` para ver las acciones sin escribir.
