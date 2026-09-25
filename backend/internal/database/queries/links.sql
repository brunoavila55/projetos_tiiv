-- name: ListarLinks :many
SELECT
    l.id, l.titulo, l.url, l.descricao, l.categoria, l.criado_por, l.criado_em,
    u.nome AS criador_nome
FROM links l
JOIN usuarios u ON u.id = l.criado_por
WHERE l.setor_id = $1
ORDER BY lower(l.categoria), lower(l.titulo);

-- name: ObterLink :one
SELECT * FROM links WHERE id = $1 AND setor_id = $2;

-- name: CriarLink :one
INSERT INTO links (titulo, url, descricao, categoria, criado_por, setor_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: AtualizarLink :one
UPDATE links
SET titulo = $2, url = $3, descricao = $4, categoria = $5, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarLink :exec
DELETE FROM links WHERE id = $1;
