-- name: ListarPlantoesIntervalo :many
-- Turnos que caem no intervalo, pelo turno ou pela folga
SELECT id, nome, tipo, cidade, periodo, inicio, fim, observacao, folga_inicio, folga_fim
FROM plantoes
WHERE setor_id = sqlc.arg(setor_id)
  AND ((fim >= sqlc.arg(de)::date AND inicio <= sqlc.arg(ate)::date)
    OR (folga_fim >= sqlc.arg(de)::date AND folga_inicio <= sqlc.arg(ate)::date))
ORDER BY inicio, tipo, cidade, periodo, nome;

-- name: ObterPlantao :one
SELECT * FROM plantoes WHERE id = $1 AND setor_id = $2;

-- name: TravarEscala :exec
-- Serializa as gravações da escala para a checagem de conflito valer
SELECT pg_advisory_xact_lock(hashtext('tiiv_plantoes'));

-- name: BuscarConflitoPlantao :one
-- A mesma pessoa (pelo nome, sem diferenciar maiúsculas) não pode ter dois
-- turnos do mesmo tipo sobrepostos no setor, nem em cidades diferentes; no
-- interno, só conflita no mesmo período (manhã e tarde no mesmo dia pode)
SELECT inicio, fim, nome, cidade, periodo
FROM plantoes
WHERE setor_id = sqlc.arg(setor_id)
  AND lower(nome) = lower(sqlc.arg(nome))
  AND tipo = sqlc.arg(tipo)
  AND periodo IS NOT DISTINCT FROM sqlc.narg(periodo)
  AND fim >= sqlc.arg(inicio)::date
  AND inicio <= sqlc.arg(fim)::date
  AND (sqlc.narg(ignorar_id)::uuid IS NULL OR id <> sqlc.narg(ignorar_id))
ORDER BY inicio
LIMIT 1;

-- name: BuscarConflitoFolga :one
-- Ninguém trabalha na própria folga: nem turno novo sobre uma folga já
-- marcada, nem folga nova sobre um turno já marcado (de qualquer tipo)
SELECT inicio, fim, folga_inicio, folga_fim, tipo, cidade, periodo
FROM plantoes
WHERE setor_id = sqlc.arg(setor_id)
  AND lower(nome) = lower(sqlc.arg(nome))
  AND (sqlc.narg(ignorar_id)::uuid IS NULL OR id <> sqlc.narg(ignorar_id))
  AND (
    (folga_inicio IS NOT NULL AND folga_fim >= sqlc.arg(inicio)::date AND folga_inicio <= sqlc.arg(fim)::date)
    OR (sqlc.narg(nova_folga_inicio)::date IS NOT NULL
        AND fim >= sqlc.narg(nova_folga_inicio)::date AND inicio <= sqlc.narg(nova_folga_fim)::date)
  )
ORDER BY inicio
LIMIT 1;

-- name: CriarPlantao :one
INSERT INTO plantoes (nome, tipo, cidade, inicio, fim, observacao, folga_inicio, folga_fim, criado_por, setor_id, periodo)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: AtualizarPlantao :one
UPDATE plantoes
SET nome = $2, tipo = $3, cidade = $4, inicio = $5, fim = $6, observacao = $7,
    folga_inicio = $8, folga_fim = $9, periodo = $10, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarPlantao :exec
DELETE FROM plantoes WHERE id = $1;

-- name: ListarPessoasPlantao :many
-- Nomes já usados na escala do setor, para sugerir ao montar turnos
SELECT DISTINCT ON (lower(nome)) nome
FROM plantoes
WHERE setor_id = $1
ORDER BY lower(nome), criado_em DESC;

-- name: ProximoPlantaoPorNome :one
-- Turno em andamento ou o próximo de quem tem este nome na escala
SELECT id, nome, tipo, cidade, periodo, inicio, fim, observacao, folga_inicio, folga_fim FROM plantoes
WHERE setor_id = sqlc.arg(setor_id) AND lower(nome) = lower(sqlc.arg(nome)) AND fim >= sqlc.arg(dia)::date
ORDER BY inicio
LIMIT 1;
