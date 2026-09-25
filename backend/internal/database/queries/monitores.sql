-- name: ListarMonitores :many
SELECT * FROM monitores
ORDER BY
    ativo DESC,
    CASE status WHEN 'offline' THEN 1 WHEN 'pendente' THEN 2 ELSE 3 END,
    lower(nome);

-- name: ObterMonitor :one
SELECT * FROM monitores WHERE id = $1;

-- name: BloquearMonitor :one
SELECT * FROM monitores WHERE id = $1 FOR UPDATE;

-- name: CriarMonitor :one
INSERT INTO monitores (nome, tipo, alvo, intervalo_seg, abrir_ticket, criado_por, setor_ticket_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AtualizarMonitor :one
UPDATE monitores
SET nome = $2, tipo = $3, alvo = $4, intervalo_seg = $5, abrir_ticket = $6, ativo = $7, setor_ticket_id = $8,
    atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: ReiniciarEstadoMonitor :exec
-- Depois de trocar o alvo ou desligar: a próxima verificação decide de novo.
UPDATE monitores
SET status = 'pendente', falhas_seguidas = 0, latencia_ms = NULL, ultimo_erro = '',
    verificado_em = NULL, status_desde = now()
WHERE id = $1;

-- name: DeletarMonitor :exec
DELETE FROM monitores WHERE id = $1;

-- name: ListarMonitoresParaVerificar :many
SELECT * FROM monitores
WHERE ativo
  AND (verificado_em IS NULL OR verificado_em <= now() - make_interval(secs => intervalo_seg));

-- name: RegistrarVerificacao :exec
UPDATE monitores
SET status = sqlc.arg(status)::varchar,
    falhas_seguidas = sqlc.arg(falhas_seguidas),
    latencia_ms = sqlc.narg(latencia_ms),
    ultimo_erro = sqlc.arg(ultimo_erro),
    verificado_em = now(),
    status_desde = CASE WHEN status = sqlc.arg(status)::varchar THEN status_desde ELSE now() END
WHERE id = sqlc.arg(id);

-- name: ResumoMonitores :one
SELECT
    count(*) FILTER (WHERE ativo) AS ativos,
    count(*) FILTER (WHERE ativo AND status = 'online') AS online,
    count(*) FILTER (WHERE ativo AND status = 'offline') AS offline
FROM monitores;

-- name: ListarMonitoresOffline :many
SELECT id, nome, tipo, alvo, ultimo_erro, status_desde FROM monitores
WHERE ativo AND status = 'offline'
ORDER BY status_desde;

-- name: AbrirQueda :one
INSERT INTO monitor_quedas (monitor_id, erro)
VALUES ($1, $2)
RETURNING id;

-- name: VincularTicketQueda :exec
UPDATE monitor_quedas SET ticket_id = $2 WHERE id = $1;

-- name: FecharQuedaAberta :exec
UPDATE monitor_quedas SET fim = now() WHERE monitor_id = $1 AND fim IS NULL;

-- name: ListarQuedasMonitor :many
SELECT q.id, q.inicio, q.fim, q.erro, tk.numero AS ticket_numero
FROM monitor_quedas q
LEFT JOIN tickets tk ON tk.id = q.ticket_id
WHERE q.monitor_id = $1
ORDER BY q.inicio DESC
LIMIT 20;

-- name: SegundosForaUltimas24h :many
-- Tempo em queda de cada monitor dentro das últimas 24 horas
SELECT
    monitor_id,
    COALESCE(sum(EXTRACT(EPOCH FROM (
        COALESCE(fim, now()) - GREATEST(inicio, now() - interval '24 hours')
    ))), 0)::float8 AS segundos
FROM monitor_quedas
WHERE COALESCE(fim, now()) > now() - interval '24 hours'
GROUP BY monitor_id;
