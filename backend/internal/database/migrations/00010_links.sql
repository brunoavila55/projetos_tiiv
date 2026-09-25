-- +goose Up
-- Links úteis da equipe (sistemas, painéis, documentação), agrupados por categoria.

CREATE TABLE links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titulo VARCHAR(120) NOT NULL,
    url VARCHAR(2000) NOT NULL,
    descricao VARCHAR(300) NOT NULL DEFAULT '',
    categoria VARCHAR(60) NOT NULL DEFAULT '',
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS links;
