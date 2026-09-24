-- name: ListarItensEstoque :many
SELECT 
    id, nome, unidade, categoria, estoque_minimo, saldo, ativo, criado_em,
    (saldo < estoque_minimo)::boolean AS abaixo_do_minimo
FROM itens_estoque
WHERE 
    (sqlc.narg('busca')::text IS NULL OR 
     unaccent(lower(nome)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%' OR
     unaccent(lower(categoria)) ILIKE '%' || unaccent(lower(sqlc.narg('busca'))) || '%')
    AND (sqlc.narg('categoria')::text IS NULL OR categoria = sqlc.narg('categoria'))
    AND (sqlc.narg('ativo')::boolean IS NULL OR ativo = sqlc.narg('ativo'))
ORDER BY nome ASC;

-- name: ListarItensAbaixoDoMinimo :many
SELECT id, nome, unidade, categoria, estoque_minimo, saldo, ativo, criado_em
FROM itens_estoque
WHERE ativo = true AND saldo < estoque_minimo
ORDER BY (estoque_minimo - saldo) DESC, nome ASC;

-- name: ListarCategoriasEstoque :many
SELECT DISTINCT categoria
FROM itens_estoque
WHERE ativo = true
ORDER BY categoria ASC;

-- name: BuscarItemEstoquePorID :one
SELECT id, nome, unidade, categoria, estoque_minimo, saldo, ativo, criado_em
FROM itens_estoque
WHERE id = $1;

-- name: BloquearItemEstoqueParaAtualizacao :one
SELECT id, nome, unidade, categoria, estoque_minimo, saldo, ativo, criado_em
FROM itens_estoque
WHERE id = $1
FOR UPDATE;

-- name: CriarItemEstoque :one
INSERT INTO itens_estoque (nome, unidade, categoria, estoque_minimo, saldo, ativo)
VALUES ($1, $2, $3, $4, 0, true)
RETURNING *;

-- name: AtualizarItemEstoque :one
UPDATE itens_estoque
SET nome = $2, unidade = $3, categoria = $4, estoque_minimo = $5, ativo = $6
WHERE id = $1
RETURNING *;

-- name: AtualizarSaldoItemEstoque :one
UPDATE itens_estoque
SET saldo = $2
WHERE id = $1
RETURNING *;

-- name: CriarMovimentacaoEstoque :one
INSERT INTO movimentacoes_estoque (item_id, tipo, quantidade, saldo_resultante, motivo, usuario_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListarMovimentacoesEstoque :many
SELECT 
    m.id, m.item_id, m.tipo, m.quantidade, m.saldo_resultante, m.motivo, m.usuario_id, m.criado_em,
    i.nome AS item_nome, i.unidade AS item_unidade,
    u.nome AS usuario_nome
FROM movimentacoes_estoque m
JOIN itens_estoque i ON i.id = m.item_id
JOIN usuarios u ON u.id = m.usuario_id
WHERE 
    (sqlc.narg('item_id')::uuid IS NULL OR m.item_id = sqlc.narg('item_id'))
    AND (sqlc.narg('usuario_id')::uuid IS NULL OR m.usuario_id = sqlc.narg('usuario_id'))
    AND (sqlc.narg('tipo')::text IS NULL OR m.tipo = sqlc.narg('tipo'))
    AND (sqlc.narg('data_inicio')::timestamptz IS NULL OR m.criado_em >= sqlc.narg('data_inicio'))
    AND (sqlc.narg('data_fim')::timestamptz IS NULL OR m.criado_em <= sqlc.narg('data_fim'))
ORDER BY m.criado_em DESC
LIMIT $1 OFFSET $2;

-- name: ObterUltimaMovimentacaoItem :one
SELECT m.id, m.item_id, m.tipo, m.quantidade, m.saldo_resultante, m.motivo, m.usuario_id, m.criado_em
FROM movimentacoes_estoque m
WHERE m.item_id = $1
ORDER BY m.criado_em DESC
LIMIT 1;

-- name: ContarMovimentacoesItem :one
SELECT count(*) FROM movimentacoes_estoque WHERE item_id = $1;

-- name: DesativarItemEstoque :exec
UPDATE itens_estoque SET ativo = false WHERE id = $1;

-- name: DeletarItemEstoque :exec
DELETE FROM itens_estoque WHERE id = $1;
