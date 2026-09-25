-- +goose Up
-- Escala de plantão e sobreaviso, por dia (inicio e fim inclusivos). Aparece
-- no calendário junto das marcações e no painel ("quem está de plantão hoje").

CREATE TABLE plantoes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    tipo VARCHAR(20) NOT NULL DEFAULT 'plantao' CHECK (tipo IN ('plantao', 'sobreaviso')),
    inicio DATE NOT NULL,
    fim DATE NOT NULL,
    observacao VARCHAR(300) NOT NULL DEFAULT '',
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_plantao_fim_maior_igual_inicio CHECK (fim >= inicio)
);

CREATE INDEX idx_plantoes_periodo ON plantoes(inicio, fim);
CREATE INDEX idx_plantoes_usuario ON plantoes(usuario_id, inicio);

-- +goose Down
DROP TABLE IF EXISTS plantoes;
