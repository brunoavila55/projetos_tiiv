-- +goose Up
-- Controle de presença de técnicos: cadastro próprio (não são usuários do
-- sistema) e registros de entrada/saída marcados por qualquer operador.

CREATE TABLE tecnicos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(255) NOT NULL,
    empresa VARCHAR(255) NOT NULL DEFAULT '',
    ativo BOOLEAN NOT NULL DEFAULT true,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_tecnicos_nome_unico ON tecnicos (lower(nome));

CREATE TABLE tecnico_registros (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tecnico_id UUID NOT NULL REFERENCES tecnicos(id) ON DELETE RESTRICT,
    entrada TIMESTAMPTZ NOT NULL,
    saida TIMESTAMPTZ,
    observacao TEXT NOT NULL DEFAULT '',
    entrada_registrada_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    saida_registrada_por UUID REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_tecnico_registro_saida_apos_entrada CHECK (saida IS NULL OR saida >= entrada)
);

-- Um técnico só pode ter uma entrada em aberto por vez.
CREATE UNIQUE INDEX idx_tecnico_registros_um_aberto ON tecnico_registros (tecnico_id) WHERE saida IS NULL;
CREATE INDEX idx_tecnico_registros_tecnico ON tecnico_registros (tecnico_id);
CREATE INDEX idx_tecnico_registros_entrada ON tecnico_registros (entrada);

-- +goose Down
DROP TABLE IF EXISTS tecnico_registros;
DROP TABLE IF EXISTS tecnicos;
