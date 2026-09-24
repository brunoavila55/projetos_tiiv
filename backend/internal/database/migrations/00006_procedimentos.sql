-- +goose Up
-- Procedimentos do setor (escritos em Markdown pelo admin) que alimentam o
-- tira-dúvidas da tela de acesso. Cada salvamento guarda uma revisão completa.

CREATE EXTENSION IF NOT EXISTS "pg_trgm";

CREATE TABLE procedimentos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titulo VARCHAR(200) NOT NULL,
    categoria VARCHAR(80) NOT NULL DEFAULT '',
    corpo TEXT NOT NULL,
    ativo BOOLEAN NOT NULL DEFAULT true,
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    atualizado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_procedimentos_ativo ON procedimentos(ativo);

CREATE TABLE procedimento_revisoes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    procedimento_id UUID NOT NULL REFERENCES procedimentos(id) ON DELETE CASCADE,
    titulo VARCHAR(200) NOT NULL,
    categoria VARCHAR(80) NOT NULL,
    corpo TEXT NOT NULL,
    ativo BOOLEAN NOT NULL,
    nota VARCHAR(200) NOT NULL DEFAULT '',
    editado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_procedimento_revisoes_proc ON procedimento_revisoes(procedimento_id, criado_em DESC);

-- Neurons da Workers AI gastos por dia (UTC, igual à cota da Cloudflare)
CREATE TABLE assistente_uso (
    dia DATE PRIMARY KEY,
    neurons DOUBLE PRECISION NOT NULL DEFAULT 0,
    perguntas INT NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS assistente_uso;
DROP TABLE IF EXISTS procedimento_revisoes;
DROP TABLE IF EXISTS procedimentos;
