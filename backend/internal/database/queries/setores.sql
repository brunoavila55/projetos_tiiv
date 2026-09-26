-- name: ListarSetores :many
SELECT
    s.id, s.nome, s.aceita_pedidos, s.modulos_desativados, s.criado_em,
    (SELECT count(*) FROM usuarios u WHERE u.setor_id = s.id AND u.ativo) AS usuarios_ativos
FROM setores s
ORDER BY lower(s.nome);

-- name: ListarSetoresPedidos :many
-- Setores que aparecem na tela de acesso (ticket e tira-dúvidas)
SELECT id, nome, modulos_desativados FROM setores
WHERE aceita_pedidos
ORDER BY lower(nome);

-- name: BuscarSetor :one
SELECT * FROM setores WHERE id = $1;

-- name: PrimeiroSetor :one
SELECT * FROM setores ORDER BY criado_em, nome LIMIT 1;

-- name: CriarSetor :one
INSERT INTO setores (nome, aceita_pedidos, modulos_desativados)
VALUES ($1, $2, $3)
RETURNING *;

-- name: AtualizarSetor :one
UPDATE setores
SET nome = $2, aceita_pedidos = $3, modulos_desativados = $4
WHERE id = $1
RETURNING *;

-- name: DeletarSetor :execrows
DELETE FROM setores WHERE id = $1;

-- name: DefinirSetorSessao :exec
UPDATE sessoes SET setor_id = $2 WHERE id = $1;

-- name: ListarEquipeSetor :many
-- Operadores ativos do setor (responsável de tarefa, participantes, escala)
SELECT u.id, u.nome, u.cor, f.atualizado_em AS foto_atualizada_em
FROM usuarios u
LEFT JOIN usuario_fotos f ON f.usuario_id = u.id
WHERE u.ativo AND u.setor_id = $1
ORDER BY u.nome ASC;

-- name: UsuarioAtivoNoSetor :one
SELECT EXISTS (
    SELECT 1 FROM usuarios WHERE id = $1 AND setor_id = $2 AND ativo
);
