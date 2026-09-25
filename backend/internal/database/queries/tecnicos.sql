-- name: ListarTecnicos :many
SELECT
    t.id, t.nome, t.empresa, t.ativo, t.criado_em,
    r.id AS registro_aberto_id,
    r.entrada AS entrada_aberta,
    r.atividades AS atividades_abertas
FROM tecnicos t
LEFT JOIN tecnico_registros r ON r.tecnico_id = t.id AND r.saida IS NULL
WHERE t.setor_id = sqlc.arg('setor_id')
  AND (sqlc.narg('ativo')::boolean IS NULL OR t.ativo = sqlc.narg('ativo'))
ORDER BY t.nome ASC;

-- name: BuscarTecnicoPorID :one
SELECT id, nome, empresa, ativo, criado_em
FROM tecnicos
WHERE id = $1 AND setor_id = $2;

-- name: CriarTecnico :one
INSERT INTO tecnicos (nome, empresa, setor_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: AtualizarTecnico :one
UPDATE tecnicos
SET nome = $2, empresa = $3, ativo = $4
WHERE id = $1 AND setor_id = sqlc.arg('setor_id')
RETURNING *;

-- name: RegistrarEntradaTecnico :one
INSERT INTO tecnico_registros (tecnico_id, entrada, observacao, entrada_registrada_por)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RegistrarSaidaTecnico :one
UPDATE tecnico_registros
SET saida = $2,
    saida_registrada_por = $3,
    observacao = CASE WHEN sqlc.arg('observacao')::text = '' THEN observacao
                      WHEN observacao = '' THEN sqlc.arg('observacao')::text
                      ELSE observacao || E'\n' || sqlc.arg('observacao')::text END,
    -- A tela de saída já vem com o texto anotado no dia: o enviado substitui
    atividades = CASE WHEN sqlc.arg('atividades')::text = '' THEN atividades
                      ELSE sqlc.arg('atividades')::text END,
    atualizado_em = now()
WHERE tecnico_id = $1 AND saida IS NULL
  AND tecnico_id IN (SELECT t.id FROM tecnicos t WHERE t.setor_id = sqlc.arg('setor_id'))
RETURNING *;

-- name: BuscarRegistroTecnicoPorID :one
SELECT r.* FROM tecnico_registros r
JOIN tecnicos t ON t.id = r.tecnico_id
WHERE r.id = $1 AND t.setor_id = $2;

-- name: AtualizarRegistroTecnico :one
UPDATE tecnico_registros
SET entrada = $2, saida = $3, observacao = $4, atualizado_em = now()
WHERE tecnico_registros.id = $1
  AND tecnico_registros.tecnico_id IN (SELECT t.id FROM tecnicos t WHERE t.setor_id = sqlc.arg('setor_id'))
RETURNING *;

-- name: AtualizarAtividadesRegistro :one
UPDATE tecnico_registros
SET atividades = $2, atualizado_em = now()
WHERE tecnico_registros.id = $1
  AND tecnico_registros.tecnico_id IN (SELECT t.id FROM tecnicos t WHERE t.setor_id = sqlc.arg('setor_id'))
RETURNING *;

-- name: DeletarRegistroTecnico :exec
DELETE FROM tecnico_registros WHERE id = $1;

-- name: ContarRegistrosTecnicos :one
SELECT count(*)
FROM tecnico_registros r
JOIN tecnicos t ON t.id = r.tecnico_id
WHERE t.setor_id = sqlc.arg('setor_id')
    AND (sqlc.narg('tecnico_id')::uuid IS NULL OR r.tecnico_id = sqlc.narg('tecnico_id'))
    AND (sqlc.narg('data_inicio')::timestamptz IS NULL OR r.entrada >= sqlc.narg('data_inicio'))
    AND (sqlc.narg('data_fim')::timestamptz IS NULL OR r.entrada < sqlc.narg('data_fim'));

-- name: ListarRegistrosTecnicos :many
SELECT
    r.id, r.tecnico_id, r.entrada, r.saida, r.observacao, r.atividades, r.atualizado_em,
    t.nome AS tecnico_nome, t.empresa AS tecnico_empresa,
    ue.nome AS entrada_registrada_por_nome,
    us.nome AS saida_registrada_por_nome
FROM tecnico_registros r
JOIN tecnicos t ON t.id = r.tecnico_id
JOIN usuarios ue ON ue.id = r.entrada_registrada_por
LEFT JOIN usuarios us ON us.id = r.saida_registrada_por
WHERE t.setor_id = sqlc.arg('setor_id')
    AND (sqlc.narg('tecnico_id')::uuid IS NULL OR r.tecnico_id = sqlc.narg('tecnico_id'))
    AND (sqlc.narg('data_inicio')::timestamptz IS NULL OR r.entrada >= sqlc.narg('data_inicio'))
    AND (sqlc.narg('data_fim')::timestamptz IS NULL OR r.entrada < sqlc.narg('data_fim'))
ORDER BY r.entrada DESC
LIMIT $1 OFFSET $2;

-- name: RelatorioTecnicos :many
-- Consolidado por técnico no período (pela data de entrada). Registros ainda
-- em aberto contam como visita, mas não somam horas.
SELECT
    t.id AS tecnico_id,
    t.nome AS tecnico_nome,
    t.empresa AS tecnico_empresa,
    count(r.id)::bigint AS total_registros,
    count(r.id) FILTER (WHERE r.saida IS NULL)::bigint AS registros_abertos,
    count(DISTINCT (r.entrada AT TIME ZONE 'America/Sao_Paulo')::date)::bigint AS dias_presentes,
    COALESCE(sum(EXTRACT(EPOCH FROM (r.saida - r.entrada))) FILTER (WHERE r.saida IS NOT NULL), 0)::bigint AS segundos_totais,
    min(r.entrada)::timestamptz AS primeira_entrada,
    max(COALESCE(r.saida, r.entrada))::timestamptz AS ultima_marcacao
FROM tecnicos t
JOIN tecnico_registros r ON r.tecnico_id = t.id
WHERE t.setor_id = sqlc.arg('setor_id')
    AND (sqlc.narg('tecnico_id')::uuid IS NULL OR r.tecnico_id = sqlc.narg('tecnico_id'))
    AND (sqlc.narg('data_inicio')::timestamptz IS NULL OR r.entrada >= sqlc.narg('data_inicio'))
    AND (sqlc.narg('data_fim')::timestamptz IS NULL OR r.entrada < sqlc.narg('data_fim'))
GROUP BY t.id, t.nome, t.empresa
ORDER BY t.nome ASC;
