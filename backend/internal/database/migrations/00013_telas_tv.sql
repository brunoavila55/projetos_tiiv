-- +goose Up
-- Telas do modo TV (telão do NOC). Cada tela tem uma chave própria, só de
-- leitura e sem expiração, para não depender de uma sessão de operador que
-- cai por inatividade. O admin revoga excluindo a tela.

CREATE TABLE telas_tv (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(80) NOT NULL,
    chave_hash VARCHAR(64) NOT NULL UNIQUE,
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    ultimo_acesso_em TIMESTAMPTZ,
    ultimo_ip VARCHAR(64) NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS telas_tv;
