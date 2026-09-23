-- name: ListarUsuariosAtivos :many
SELECT id, nome, cor
FROM usuarios
WHERE ativo = true
ORDER BY nome ASC;

-- name: ListarTodosUsuarios :many
SELECT id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em
FROM usuarios
ORDER BY nome ASC;

-- name: BuscarUsuarioPorID :one
SELECT id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em
FROM usuarios
WHERE id = $1;

-- name: BuscarUsuarioPorIDComPin :one
SELECT id, nome, cor, pin_hash, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em
FROM usuarios
WHERE id = $1;

-- name: ContarUsuarios :one
SELECT count(*)
FROM usuarios;

-- name: ContarAdminsAtivos :one
SELECT count(*)
FROM usuarios
WHERE papel = 'admin' AND ativo = true;

-- name: CriarUsuario :one
INSERT INTO usuarios (nome, cor, pin_hash, papel, ativo)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em;

-- name: AtualizarUsuario :one
UPDATE usuarios
SET nome = $2, cor = $3, papel = $4, ativo = $5
WHERE id = $1
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em;

-- name: AtualizarPin :one
UPDATE usuarios
SET pin_hash = $2
WHERE id = $1
RETURNING id;

-- name: IncrementarTentativasFalhas :one
UPDATE usuarios
SET tentativas_falhas = tentativas_falhas + 1,
    bloqueado_ate = CASE 
        WHEN tentativas_falhas + 1 >= 5 THEN now() + interval '5 minutes'
        ELSE bloqueado_ate
    END
WHERE id = $1
RETURNING tentativas_falhas, bloqueado_ate;

-- name: ZerarTentativasFalhas :exec
UPDATE usuarios
SET tentativas_falhas = 0, bloqueado_ate = NULL
WHERE id = $1;

-- name: DesbloquearUsuario :one
UPDATE usuarios
SET tentativas_falhas = 0, bloqueado_ate = NULL
WHERE id = $1
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em;
