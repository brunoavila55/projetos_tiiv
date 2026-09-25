-- name: ListarEventosIntervalo :many
SELECT 
    e.id, e.titulo, e.descricao, e.inicio, e.fim, e.dia_inteiro, e.criado_por, e.criado_em, e.atualizado_em,
    e.recorrencia, e.recorrencia_fim,
    u.nome AS criador_nome, u.cor AS criador_cor
FROM eventos e
JOIN usuarios u ON u.id = e.criado_por
WHERE e.setor_id = $3
  AND ((e.fim >= $1 AND e.inicio <= $2)
   OR (e.recorrencia != 'nenhuma' AND e.inicio <= $2 AND (e.recorrencia_fim IS NULL OR e.recorrencia_fim >= $1)))
ORDER BY e.inicio ASC;

-- name: ListarParticipantesPorEvento :many
SELECT ep.evento_id, ep.usuario_id, u.nome, u.cor
FROM evento_participantes ep
JOIN usuarios u ON u.id = ep.usuario_id
WHERE ep.evento_id = $1;

-- name: ListarParticipantesPorEventos :many
SELECT ep.evento_id, ep.usuario_id, u.nome, u.cor
FROM evento_participantes ep
JOIN usuarios u ON u.id = ep.usuario_id
WHERE ep.evento_id = ANY($1::uuid[]);

-- name: BuscarEventoPorID :one
SELECT e.id, e.titulo, e.descricao, e.inicio, e.fim, e.dia_inteiro, e.criado_por, e.criado_em, e.atualizado_em, e.recorrencia, e.recorrencia_fim
FROM eventos e
WHERE e.id = $1 AND e.setor_id = $2;

-- name: CriarEvento :one
INSERT INTO eventos (titulo, descricao, inicio, fim, dia_inteiro, criado_por, recorrencia, recorrencia_fim, setor_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: AdicionarParticipanteEvento :exec
-- Só entra quem é do setor do evento (ou quem o criou)
INSERT INTO evento_participantes (evento_id, usuario_id)
SELECT e.id, u.id
FROM eventos e
JOIN usuarios u ON u.id = sqlc.arg('usuario_id')
WHERE e.id = sqlc.arg('evento_id') AND (u.setor_id = e.setor_id OR u.id = e.criado_por)
ON CONFLICT DO NOTHING;

-- name: RemoverParticipantesEvento :exec
DELETE FROM evento_participantes
WHERE evento_id = $1;

-- name: AtualizarEvento :one
UPDATE eventos
SET titulo = $2, descricao = $3, inicio = $4, fim = $5, dia_inteiro = $6, recorrencia = $7, recorrencia_fim = $8, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: DeletarEvento :exec
DELETE FROM eventos
WHERE id = $1;
