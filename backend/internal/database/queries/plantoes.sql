-- name: ListarPlantoesIntervalo :many
SELECT
    p.id, p.usuario_id, p.tipo, p.inicio, p.fim, p.observacao,
    u.nome AS usuario_nome, u.cor AS usuario_cor
FROM plantoes p
JOIN usuarios u ON u.id = p.usuario_id
WHERE p.fim >= sqlc.arg(de)::date AND p.inicio <= sqlc.arg(ate)::date
ORDER BY p.inicio, p.tipo, u.nome;

-- name: ObterPlantao :one
SELECT * FROM plantoes WHERE id = $1;

-- name: TravarEscala :exec
-- Serializa as gravações da escala para a checagem de conflito valer
SELECT pg_advisory_xact_lock(hashtext('tiiv_plantoes'));

-- name: BuscarConflitoPlantao :one
-- A mesma pessoa não pode ter dois turnos do mesmo tipo sobrepostos
SELECT p.inicio, p.fim, u.nome AS usuario_nome
FROM plantoes p
JOIN usuarios u ON u.id = p.usuario_id
WHERE p.usuario_id = sqlc.arg(usuario_id)
  AND p.tipo = sqlc.arg(tipo)
  AND p.fim >= sqlc.arg(inicio)::date
  AND p.inicio <= sqlc.arg(fim)::date
  AND (sqlc.narg(ignorar_id)::uuid IS NULL OR p.id <> sqlc.narg(ignorar_id))
ORDER BY p.inicio
LIMIT 1;

-- name: CriarPlantao :one
INSERT INTO plantoes (usuario_id, tipo, inicio, fim, observacao, criado_por)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: AtualizarPlantao :one
UPDATE plantoes
SET usuario_id = $2, tipo = $3, inicio = $4, fim = $5, observacao = $6, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarPlantao :exec
DELETE FROM plantoes WHERE id = $1;

-- name: ProximoPlantaoUsuario :one
-- Turno em andamento ou o próximo da pessoa
SELECT id, tipo, inicio, fim FROM plantoes
WHERE usuario_id = sqlc.arg(usuario_id) AND fim >= sqlc.arg(dia)::date
ORDER BY inicio
LIMIT 1;
