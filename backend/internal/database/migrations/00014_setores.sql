-- +goose Up
-- Setores (NOC, Agendamento...): cada operador pertence a um setor e só vê
-- tickets, tarefas, calendário, plantão, procedimentos, avisos, links e telas
-- de TV do próprio setor. Monitor, estoque e técnicos continuam compartilhados.
-- O superadmin cria setores, move pessoas entre eles e escolhe, na sessão,
-- qual setor está vendo. Os dados que já existiam ficam no setor "NOC".

CREATE TABLE setores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(60) NOT NULL,
    -- Aparece na tela de acesso para abrir ticket e no tira-dúvidas
    aceita_pedidos BOOLEAN NOT NULL DEFAULT true,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_setores_nome ON setores (lower(nome));

INSERT INTO setores (nome) VALUES ('NOC');

-- Usuários: setor obrigatório e o novo papel global
ALTER TABLE usuarios ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE usuarios SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE usuarios ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_usuarios_setor ON usuarios(setor_id);

ALTER TABLE usuarios DROP CONSTRAINT usuarios_papel_check;
ALTER TABLE usuarios ADD CONSTRAINT usuarios_papel_check CHECK (papel IN ('superadmin', 'admin', 'usuario'));
UPDATE usuarios SET papel = 'superadmin' WHERE papel = 'admin';

-- Setor que o superadmin está vendo nesta sessão (NULL = o dele)
ALTER TABLE sessoes ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE SET NULL;

-- Dados separados por setor
ALTER TABLE tickets ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE tickets SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE tickets ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_tickets_setor_status ON tickets(setor_id, status);

ALTER TABLE tarefas ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE tarefas SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE tarefas ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_tarefas_setor ON tarefas(setor_id);

ALTER TABLE eventos ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE eventos SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE eventos ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_eventos_setor ON eventos(setor_id, inicio);

ALTER TABLE plantoes ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE plantoes SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE plantoes ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_plantoes_setor ON plantoes(setor_id, inicio, fim);

ALTER TABLE procedimentos ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE procedimentos SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE procedimentos ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_procedimentos_setor ON procedimentos(setor_id);

ALTER TABLE avisos ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE avisos SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE avisos ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_avisos_setor ON avisos(setor_id);

ALTER TABLE links ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE links SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE links ALTER COLUMN setor_id SET NOT NULL;
CREATE INDEX idx_links_setor ON links(setor_id);

ALTER TABLE telas_tv ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE telas_tv SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE telas_tv ALTER COLUMN setor_id SET NOT NULL;

-- Monitores são de todos, mas o ticket de queda cai na fila de um setor
ALTER TABLE monitores ADD COLUMN setor_ticket_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE monitores SET setor_ticket_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE monitores ALTER COLUMN setor_ticket_id SET NOT NULL;

-- +goose Down
ALTER TABLE monitores DROP COLUMN IF EXISTS setor_ticket_id;
ALTER TABLE telas_tv DROP COLUMN IF EXISTS setor_id;
ALTER TABLE links DROP COLUMN IF EXISTS setor_id;
ALTER TABLE avisos DROP COLUMN IF EXISTS setor_id;
ALTER TABLE procedimentos DROP COLUMN IF EXISTS setor_id;
ALTER TABLE plantoes DROP COLUMN IF EXISTS setor_id;
ALTER TABLE eventos DROP COLUMN IF EXISTS setor_id;
ALTER TABLE tarefas DROP COLUMN IF EXISTS setor_id;
ALTER TABLE tickets DROP COLUMN IF EXISTS setor_id;
ALTER TABLE sessoes DROP COLUMN IF EXISTS setor_id;

UPDATE usuarios SET papel = 'admin' WHERE papel = 'superadmin';
ALTER TABLE usuarios DROP CONSTRAINT usuarios_papel_check;
ALTER TABLE usuarios ADD CONSTRAINT usuarios_papel_check CHECK (papel IN ('admin', 'usuario'));
ALTER TABLE usuarios DROP COLUMN IF EXISTS setor_id;

DROP TABLE IF EXISTS setores;
