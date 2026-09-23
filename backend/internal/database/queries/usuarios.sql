-- name: ListarUsuariosAtivos :many
SELECT u.id, u.nome, u.cor, f.atualizado_em AS foto_atualizada_em
FROM usuarios u
LEFT JOIN usuario_fotos f ON f.usuario_id = u.id
WHERE u.ativo = true
ORDER BY u.nome ASC;

-- name: ListarTodosUsuarios :many
SELECT u.id, u.nome, u.cor, u.papel, u.ativo, u.tentativas_falhas, u.bloqueado_ate, u.criado_em, u.tema,
       f.atualizado_em AS foto_atualizada_em
FROM usuarios u
LEFT JOIN usuario_fotos f ON f.usuario_id = u.id
ORDER BY u.nome ASC;

-- name: BuscarUsuarioPorID :one
SELECT id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema
FROM usuarios
WHERE id = $1;

-- name: BuscarUsuarioPorIDComPin :one
SELECT id, nome, cor, pin_hash, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema
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
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema;

-- name: AtualizarUsuario :one
UPDATE usuarios
SET nome = $2, cor = $3, papel = $4, ativo = $5
WHERE id = $1
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema;

-- name: AtualizarTemaUsuario :one
UPDATE usuarios
SET tema = $2
WHERE id = $1
RETURNING id, tema;

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
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema;

-- name: ObterFotoUsuario :one
SELECT conteudo, mime, atualizado_em
FROM usuario_fotos
WHERE usuario_id = $1;

-- name: ObterVersaoFotoUsuario :one
SELECT atualizado_em
FROM usuario_fotos
WHERE usuario_id = $1;

-- name: SalvarFotoUsuario :one
INSERT INTO usuario_fotos (usuario_id, conteudo, mime, atualizado_em)
VALUES ($1, $2, $3, now())
ON CONFLICT (usuario_id) DO UPDATE
SET conteudo = EXCLUDED.conteudo, mime = EXCLUDED.mime, atualizado_em = now()
RETURNING atualizado_em;

-- name: RemoverFotoUsuario :exec
DELETE FROM usuario_fotos
WHERE usuario_id = $1;
