-- name: ListarProcedimentos :many
SELECT
    p.id, p.titulo, p.categoria, p.ativo, p.atualizado_em,
    u.nome AS atualizado_por_nome
FROM procedimentos p
JOIN usuarios u ON u.id = p.atualizado_por
WHERE
    (sqlc.narg('busca')::text IS NULL OR
     unaccent(lower(p.titulo || ' ' || p.categoria || ' ' || p.corpo)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%')
    AND (sqlc.narg('categoria')::text IS NULL OR p.categoria = sqlc.narg('categoria'))
    AND (sqlc.narg('ativo')::boolean IS NULL OR p.ativo = sqlc.narg('ativo'))
ORDER BY p.ativo DESC, p.categoria ASC, p.titulo ASC;

-- name: ListarCategoriasProcedimentos :many
SELECT DISTINCT categoria
FROM procedimentos
WHERE categoria <> ''
ORDER BY categoria ASC;

-- name: BuscarProcedimentoPorID :one
SELECT
    p.id, p.titulo, p.categoria, p.corpo, p.ativo, p.criado_em, p.atualizado_em,
    u.nome AS atualizado_por_nome
FROM procedimentos p
JOIN usuarios u ON u.id = p.atualizado_por
WHERE p.id = $1;

-- name: CriarProcedimento :one
INSERT INTO procedimentos (titulo, categoria, corpo, ativo, criado_por, atualizado_por)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING *;

-- name: AtualizarProcedimento :one
UPDATE procedimentos
SET titulo = $2, categoria = $3, corpo = $4, ativo = $5, atualizado_por = $6, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: CriarRevisaoProcedimento :exec
INSERT INTO procedimento_revisoes (procedimento_id, titulo, categoria, corpo, ativo, nota, editado_por)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListarRevisoesProcedimento :many
SELECT
    r.id, r.titulo, r.categoria, r.corpo, r.ativo, r.nota, r.criado_em,
    u.nome AS editado_por_nome
FROM procedimento_revisoes r
JOIN usuarios u ON u.id = r.editado_por
WHERE r.procedimento_id = $1
ORDER BY r.criado_em DESC;

-- name: BuscarRevisaoProcedimento :one
SELECT * FROM procedimento_revisoes
WHERE id = $1 AND procedimento_id = $2;

-- name: BuscarProcedimentosRelevantes :many
-- Busca para o tira-dúvidas: full-text em português (termos em OU) somado à
-- semelhança por trigramas, que tolera erros de digitação.
WITH consulta AS (
    SELECT
        unaccent(lower(sqlc.arg('texto')::text)) AS txt,
        NULLIF(replace(plainto_tsquery('portuguese', unaccent(lower(sqlc.arg('texto')::text)))::text, ' & ', ' | '), '')::tsquery AS tsq
)
SELECT p.id, p.titulo, p.categoria, p.corpo, pontos.relevancia
FROM procedimentos p
CROSS JOIN consulta c
CROSS JOIN LATERAL (
    SELECT (
        COALESCE(ts_rank(
            setweight(to_tsvector('portuguese', unaccent(lower(p.titulo))), 'A') ||
            setweight(to_tsvector('portuguese', unaccent(lower(p.categoria || ' ' || p.corpo))), 'B'),
            c.tsq
        ), 0) +
        word_similarity(c.txt, unaccent(lower(p.titulo || ' ' || p.corpo)))
    )::float8 AS relevancia
) pontos
WHERE p.ativo
ORDER BY pontos.relevancia DESC
LIMIT sqlc.arg('limite')::int;

-- name: ObterUsoAssistente :one
SELECT neurons::float8 AS neurons, perguntas FROM assistente_uso
WHERE dia = (now() AT TIME ZONE 'UTC')::date;

-- name: RegistrarUsoAssistente :exec
INSERT INTO assistente_uso (dia, neurons, perguntas)
VALUES ((now() AT TIME ZONE 'UTC')::date, sqlc.arg('neurons')::float8, 1)
ON CONFLICT (dia) DO UPDATE
SET neurons = assistente_uso.neurons + EXCLUDED.neurons,
    perguntas = assistente_uso.perguntas + 1;
