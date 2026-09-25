-- +goose Up
-- Mural de avisos do painel: recados fixados para a equipe, que somem
-- sozinhos depois de expira_em (sem expira_em, ficam até alguém excluir).

CREATE TABLE avisos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titulo VARCHAR(200) NOT NULL,
    mensagem TEXT NOT NULL DEFAULT '',
    nivel VARCHAR(20) NOT NULL DEFAULT 'info' CHECK (nivel IN ('info', 'atencao', 'critico')),
    expira_em TIMESTAMPTZ,
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_avisos_expira_em ON avisos(expira_em);

-- +goose Down
DROP TABLE IF EXISTS avisos;
