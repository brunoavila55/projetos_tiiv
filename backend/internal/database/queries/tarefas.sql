-- name: ListarTarefas :many
SELECT 
    t.id, t.titulo, t.descricao, t.status, t.prioridade, t.prazo, 
    t.criado_por, t.responsavel_id, t.concluida_em, t.criado_em, t.atualizado_em,
    uc.nome AS criador_nome, uc.cor AS criador_cor,
    ur.nome AS responsavel_nome, ur.cor AS responsavel_cor
FROM tarefas t
JOIN usuarios uc ON uc.id = t.criado_por
LEFT JOIN usuarios ur ON ur.id = t.responsavel_id
WHERE 
    (sqlc.narg('responsavel_id')::uuid IS NULL OR t.responsavel_id = sqlc.narg('responsavel_id'))
    AND (sqlc.narg('criado_por')::uuid IS NULL OR t.criado_por = sqlc.narg('criado_por'))
    AND (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status'))
ORDER BY 
    CASE WHEN t.status = 'concluida' THEN 1 ELSE 0 END ASC,
    CASE WHEN t.prazo IS NOT NULL AND t.prazo < now() AND t.status != 'concluida' THEN 0 ELSE 1 END ASC,
    t.prazo ASC NULLS LAST,
    CASE t.prioridade 
        WHEN 'alta' THEN 1 
        WHEN 'media' THEN 2 
        WHEN 'baixa' THEN 3 
        ELSE 4 
    END ASC,
    t.criado_em DESC;

-- name: ListarTarefasPendentesUsuario :many
SELECT 
    t.id, t.titulo, t.descricao, t.status, t.prioridade, t.prazo, 
    t.criado_por, t.responsavel_id, t.concluida_em, t.criado_em, t.atualizado_em,
    uc.nome AS criador_nome, uc.cor AS criador_cor
FROM tarefas t
JOIN usuarios uc ON uc.id = t.criado_por
WHERE t.responsavel_id = $1 AND t.status != 'concluida'
ORDER BY 
    CASE WHEN t.prazo IS NOT NULL AND t.prazo < now() THEN 0 ELSE 1 END ASC,
    t.prazo ASC NULLS LAST,
    CASE t.prioridade 
        WHEN 'alta' THEN 1 
        WHEN 'media' THEN 2 
        WHEN 'baixa' THEN 3 
        ELSE 4 
    END ASC;

-- name: BuscarTarefaPorID :one
SELECT 
    t.id, t.titulo, t.descricao, t.status, t.prioridade, t.prazo, 
    t.criado_por, t.responsavel_id, t.concluida_em, t.criado_em, t.atualizado_em,
    uc.nome AS criador_nome, uc.cor AS criador_cor,
    ur.nome AS responsavel_nome, ur.cor AS responsavel_cor
FROM tarefas t
JOIN usuarios uc ON uc.id = t.criado_por
LEFT JOIN usuarios ur ON ur.id = t.responsavel_id
WHERE t.id = $1;

-- name: CriarTarefa :one
INSERT INTO tarefas (titulo, descricao, prioridade, prazo, criado_por, responsavel_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: AtualizarTarefa :one
UPDATE tarefas
SET titulo = $2, descricao = $3, prioridade = $4, prazo = $5, responsavel_id = $6, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: AtualizarStatusTarefa :one
UPDATE tarefas
SET status = $2,
    concluida_em = CASE 
        WHEN $2 = 'concluida' THEN now() 
        ELSE NULL 
    END,
    atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarTarefa :exec
DELETE FROM tarefas
WHERE id = $1;
