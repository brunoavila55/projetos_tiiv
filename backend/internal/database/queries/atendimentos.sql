-- name: ListarAtendimentos :many
SELECT 
    a.id, a.cliente_nome, a.descricao, a.data_atendimento, a.usuario_id, a.criado_em, a.atualizado_em,
    u.nome AS usuario_nome, u.cor AS usuario_cor
FROM atendimentos a
JOIN usuarios u ON u.id = a.usuario_id
WHERE 
    (sqlc.narg('busca')::text IS NULL OR 
     unaccent(lower(a.cliente_nome)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%' OR
     unaccent(lower(a.descricao)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%')
    AND (sqlc.narg('usuario_id')::uuid IS NULL OR a.usuario_id = sqlc.narg('usuario_id'))
    AND (sqlc.narg('data_inicio')::timestamptz IS NULL OR a.data_atendimento >= sqlc.narg('data_inicio'))
    AND (sqlc.narg('data_fim')::timestamptz IS NULL OR a.data_atendimento <= sqlc.narg('data_fim'))
ORDER BY a.data_atendimento DESC
LIMIT $1 OFFSET $2;

-- name: ContarAtendimentos :one
SELECT count(*)
FROM atendimentos a
WHERE 
    (sqlc.narg('busca')::text IS NULL OR 
     unaccent(lower(a.cliente_nome)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%' OR
     unaccent(lower(a.descricao)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%')
    AND (sqlc.narg('usuario_id')::uuid IS NULL OR a.usuario_id = sqlc.narg('usuario_id'))
    AND (sqlc.narg('data_inicio')::timestamptz IS NULL OR a.data_atendimento >= sqlc.narg('data_inicio'))
    AND (sqlc.narg('data_fim')::timestamptz IS NULL OR a.data_atendimento <= sqlc.narg('data_fim'));

-- name: BuscarAtendimentoPorID :one
SELECT 
    a.id, a.cliente_nome, a.descricao, a.data_atendimento, a.usuario_id, a.criado_em, a.atualizado_em,
    u.nome AS usuario_nome, u.cor AS usuario_cor
FROM atendimentos a
JOIN usuarios u ON u.id = a.usuario_id
WHERE a.id = $1;

-- name: CriarAtendimento :one
INSERT INTO atendimentos (cliente_nome, descricao, data_atendimento, usuario_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: AtualizarAtendimento :one
UPDATE atendimentos
SET cliente_nome = $2, descricao = $3, data_atendimento = $4, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarAtendimento :exec
DELETE FROM atendimentos
WHERE id = $1;
