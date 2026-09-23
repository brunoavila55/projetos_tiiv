-- +goose Up
-- Foto de perfil dos operadores. Fica numa tabela separada para não pesar
-- nas consultas de usuarios; o navegador já envia a imagem reduzida.

CREATE TABLE usuario_fotos (
    usuario_id UUID PRIMARY KEY REFERENCES usuarios(id) ON DELETE CASCADE,
    conteudo BYTEA NOT NULL,
    mime VARCHAR(20) NOT NULL CHECK (mime IN ('image/jpeg', 'image/png', 'image/webp')),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS usuario_fotos;
