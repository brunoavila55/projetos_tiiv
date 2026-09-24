-- +goose Up
-- Tickets abertos sem login (pela tela de acesso). Ao ser resgatado por um
-- operador, o ticket vira uma tarefa atribuída a ele.

CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    numero BIGSERIAL NOT NULL UNIQUE,
    solicitante_nome VARCHAR(120) NOT NULL,
    titulo VARCHAR(200) NOT NULL,
    descricao TEXT NOT NULL,
    prioridade VARCHAR(20) NOT NULL DEFAULT 'media' CHECK (prioridade IN ('baixa', 'media', 'alta')),
    status VARCHAR(20) NOT NULL DEFAULT 'aberto' CHECK (status IN ('aberto', 'resgatado', 'descartado')),
    tarefa_id UUID REFERENCES tarefas(id) ON DELETE SET NULL,
    tratado_por UUID REFERENCES usuarios(id) ON DELETE RESTRICT,
    tratado_em TIMESTAMPTZ,
    origem_ip VARCHAR(64) NOT NULL DEFAULT '',
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tickets_status ON tickets(status);
CREATE INDEX idx_tickets_criado_em ON tickets(criado_em DESC);
CREATE INDEX idx_tickets_tarefa_id ON tickets(tarefa_id);

-- +goose Down
DROP TABLE IF EXISTS tickets;
