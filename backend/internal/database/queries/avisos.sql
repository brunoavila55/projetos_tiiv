-- name: ListarAvisosAtivos :many
SELECT
    a.id, a.titulo, a.mensagem, a.nivel, a.expira_em, a.criado_por,
    a.criado_em, a.atualizado_em,
    u.nome AS criador_nome, u.cor AS criador_cor
FROM avisos a
JOIN usuarios u ON u.id = a.criado_por
WHERE a.setor_id = $1 AND (a.expira_em IS NULL OR a.expira_em > now())
ORDER BY
    CASE a.nivel WHEN 'critico' THEN 1 WHEN 'atencao' THEN 2 ELSE 3 END,
    a.criado_em DESC
LIMIT 50;

-- name: ObterAviso :one
SELECT * FROM avisos WHERE id = $1 AND setor_id = $2;

-- name: CriarAviso :one
INSERT INTO avisos (titulo, mensagem, nivel, expira_em, criado_por, setor_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: AtualizarAviso :one
UPDATE avisos
SET titulo = $2, mensagem = $3, nivel = $4, expira_em = $5, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarAviso :exec
DELETE FROM avisos WHERE id = $1;
