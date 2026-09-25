-- +goose Up
-- Monitor de disponibilidade: o servidor verifica cada alvo (ping, porta TCP
-- ou URL HTTP) no intervalo configurado. Duas falhas seguidas marcam o monitor
-- como fora do ar e abrem uma queda; a primeira resposta boa fecha a queda.

CREATE TABLE monitores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(120) NOT NULL,
    tipo VARCHAR(10) NOT NULL CHECK (tipo IN ('ping', 'tcp', 'http')),
    alvo VARCHAR(500) NOT NULL,
    intervalo_seg INTEGER NOT NULL DEFAULT 60 CHECK (intervalo_seg BETWEEN 30 AND 3600),
    abrir_ticket BOOLEAN NOT NULL DEFAULT false,
    ativo BOOLEAN NOT NULL DEFAULT true,
    status VARCHAR(20) NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente', 'online', 'offline')),
    falhas_seguidas INTEGER NOT NULL DEFAULT 0,
    latencia_ms INTEGER,
    ultimo_erro TEXT NOT NULL DEFAULT '',
    verificado_em TIMESTAMPTZ,
    status_desde TIMESTAMPTZ NOT NULL DEFAULT now(),
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE monitor_quedas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id UUID NOT NULL REFERENCES monitores(id) ON DELETE CASCADE,
    inicio TIMESTAMPTZ NOT NULL DEFAULT now(),
    fim TIMESTAMPTZ,
    erro TEXT NOT NULL DEFAULT '',
    ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL
);

CREATE INDEX idx_monitor_quedas_monitor ON monitor_quedas(monitor_id, inicio DESC);
-- No máximo uma queda aberta por monitor
CREATE UNIQUE INDEX idx_monitor_quedas_aberta ON monitor_quedas(monitor_id) WHERE fim IS NULL;

-- +goose Down
DROP TABLE IF EXISTS monitor_quedas;
DROP TABLE IF EXISTS monitores;
