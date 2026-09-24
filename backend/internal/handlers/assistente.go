package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/response"
)

const (
	perguntasPorJanela     = 20
	janelaPerguntas        = 10 * time.Minute
	maxMensagensConversa   = 20
	maxCaracteresMensagem  = 1000
	mensagensEnviadasAoLLM = 6    // só o fim da conversa vai para o modelo
	procedimentosNoPrompt  = 5    // os mais parecidos com a pergunta
	relevanciaMinima       = 0.4  // abaixo disso é ruído dos trigramas ("oi" dá ~0.33)
	maxCorpoNoPrompt       = 3000 // caracteres de cada procedimento
	maxTokensResposta      = 500
	timeoutWorkersAI       = 12 * time.Second // abaixo do WriteTimeout do servidor
)

const semProcedimento = "Não encontrei um procedimento para isso. Tente descrever com outras palavras ou, se preferir, abra um ticket pelo formulário."

type AssistenteHandler struct {
	db      *database.DB
	cfg     *config.Config
	http    *http.Client
	limites *limitadorPorIP
}

func NewAssistenteHandler(db *database.DB, cfg *config.Config) *AssistenteHandler {
	return &AssistenteHandler{
		db:      db,
		cfg:     cfg,
		http:    &http.Client{Timeout: timeoutWorkersAI},
		limites: novoLimitadorPorIP(perguntasPorJanela, janelaPerguntas),
	}
}

func (h *AssistenteHandler) configurado() bool {
	return h.cfg.CFAccountID != "" && h.cfg.CFAPIToken != ""
}

// cotaEsgotada diz se os neurons gastos hoje (UTC) já chegaram ao limite
func (h *AssistenteHandler) cotaEsgotada(ctx context.Context) (bool, error) {
	uso, err := h.db.Queries.ObterUsoAssistente(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return uso.Neurons >= h.cfg.AssistenteNeuronsDia, nil
}

// Status: GET /api/assistente/status (sem login)
func (h *AssistenteHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !h.configurado() {
		response.JSON(w, http.StatusOK, map[string]any{"disponivel": false, "motivo": "desligado"})
		return
	}
	esgotada, err := h.cotaEsgotada(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao consultar o assistente")
		return
	}
	if esgotada {
		response.JSON(w, http.StatusOK, map[string]any{"disponivel": false, "motivo": "cota"})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"disponivel": true})
}

type MensagemConversa struct {
	Papel string `json:"papel"` // usuario | assistente
	Texto string `json:"texto"`
}

type PerguntarRequest struct {
	Mensagens []MensagemConversa `json:"mensagens"`
}

type FonteResposta struct {
	ID      string  `json:"id"`
	Titulo  string  `json:"titulo"`
	Contato *string `json:"contato"`
}

type PerguntarResponse struct {
	Resposta string          `json:"resposta"`
	Fontes   []FonteResposta `json:"fontes"`
}

// Perguntar: POST /api/assistente/publico (sem login)
// A conversa não é gravada: o navegador manda o histórico a cada pergunta.
func (h *AssistenteHandler) Perguntar(w http.ResponseWriter, r *http.Request) {
	if !h.configurado() {
		response.JSONError(w, http.StatusServiceUnavailable, "o tira-dúvidas não está disponível")
		return
	}

	var req PerguntarRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if msg := validarConversa(req.Mensagens); msg != "" {
		response.JSONError(w, http.StatusBadRequest, msg)
		return
	}

	if !h.limites.permitir(ipDaRequisicao(r)) {
		response.JSONError(w, http.StatusTooManyRequests, "muitas perguntas seguidas; aguarde alguns minutos")
		return
	}

	esgotada, err := h.cotaEsgotada(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao consultar o assistente")
		return
	}
	if esgotada {
		response.JSONError(w, http.StatusServiceUnavailable, "o tira-dúvidas atingiu o limite de hoje; abra um ticket pelo formulário")
		return
	}

	procs, err := h.db.Queries.BuscarProcedimentosRelevantes(r.Context(), sqlc.BuscarProcedimentosRelevantesParams{
		Texto:  textoDeBusca(req.Mensagens),
		Limite: procedimentosNoPrompt,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar procedimentos")
		return
	}
	relevantes := procs[:0]
	for _, p := range procs {
		if p.Relevancia >= relevanciaMinima {
			relevantes = append(relevantes, p)
		}
	}
	// Sem procedimento parecido não há o que o modelo responder: poupa a cota
	if len(relevantes) == 0 {
		response.JSON(w, http.StatusOK, PerguntarResponse{Resposta: semProcedimento, Fontes: []FonteResposta{}})
		return
	}

	conteudo, uso, err := h.chamarWorkersAI(r.Context(), montarMensagensLLM(req.Mensagens, relevantes))
	if err != nil {
		slog.Error("falha ao consultar a Workers AI", "erro", err)
		response.JSONError(w, http.StatusBadGateway, "o tira-dúvidas não respondeu; tente de novo ou abra um ticket")
		return
	}

	neurons := float64(uso.PromptTokens)*h.cfg.NeuronsEntradaPorM/1e6 + float64(uso.CompletionTokens)*h.cfg.NeuronsSaidaPorM/1e6
	if err := h.db.Queries.RegistrarUsoAssistente(r.Context(), neurons); err != nil {
		slog.Error("falha ao registrar uso do assistente", "erro", err)
	}

	resposta, indices := separarFontes(limparPensamento(conteudo))
	if resposta == "" {
		resposta = semProcedimento
	}
	fontes := []FonteResposta{}
	for _, i := range indices {
		if i < 1 || i > len(relevantes) {
			continue
		}
		p := relevantes[i-1]
		fontes = append(fontes, FonteResposta{
			ID:      database.UUIDToString(p.ID),
			Titulo:  p.Titulo,
			Contato: extrairContato(p.Corpo),
		})
	}

	response.JSON(w, http.StatusOK, PerguntarResponse{Resposta: resposta, Fontes: fontes})
}

func validarConversa(msgs []MensagemConversa) string {
	if len(msgs) == 0 {
		return "envie ao menos uma pergunta"
	}
	if len(msgs) > maxMensagensConversa {
		return "a conversa ficou longa demais; comece uma nova"
	}
	for _, m := range msgs {
		if m.Papel != "usuario" && m.Papel != "assistente" {
			return "mensagem inválida"
		}
		// Respostas do assistente podem passar do limite de uma pergunta
		if utf8.RuneCountInString(m.Texto) > maxCaracteresMensagem*4 {
			return "mensagem longa demais"
		}
	}
	ultima := msgs[len(msgs)-1]
	if ultima.Papel != "usuario" || strings.TrimSpace(ultima.Texto) == "" {
		return "envie uma pergunta"
	}
	if utf8.RuneCountInString(ultima.Texto) > maxCaracteresMensagem {
		return fmt.Sprintf("a pergunta deve ter no máximo %d caracteres", maxCaracteresMensagem)
	}
	return ""
}

// textoDeBusca junta as duas últimas perguntas, para que um "e se ele não
// atender?" ainda encontre o procedimento da pergunta anterior.
func textoDeBusca(msgs []MensagemConversa) string {
	var partes []string
	for i := len(msgs) - 1; i >= 0 && len(partes) < 2; i-- {
		if msgs[i].Papel == "usuario" {
			partes = append(partes, msgs[i].Texto)
		}
	}
	return strings.Join(partes, " ")
}

const instrucoesAssistente = `Você é o tira-dúvidas do setor NOC, na tela onde as pessoas reportam problemas.
Responda em português do Brasil, de forma curta e direta (até 6 linhas), usando Markdown simples.
Use APENAS os procedimentos e informações abaixo. Não invente passos, números, nomes, telefones ou ramais.
Responda só o que foi perguntado. Quando for uma informação simples, dê a informação e pare.
Não mencione cartões, contatos ou "mais detalhes" se a pessoa não pediu. A seção "Contato" de um procedimento já é mostrada à parte: não repita telefone nem ramal dela.
Se nada abaixo responder à dúvida, diga que não encontrou e sugira abrir um ticket pelo formulário da tela.
Na última linha escreva exatamente "FONTES:" seguido dos números dos procedimentos que você usou, separados por vírgula (ex.: "FONTES: 2"), ou "FONTES: nenhum".

Procedimentos:
`

type mensagemLLM struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func montarMensagensLLM(msgs []MensagemConversa, procs []sqlc.BuscarProcedimentosRelevantesRow) []mensagemLLM {
	var sb strings.Builder
	sb.WriteString(instrucoesAssistente)
	for i, p := range procs {
		corpo := p.Corpo
		if utf8.RuneCountInString(corpo) > maxCorpoNoPrompt {
			corpo = string([]rune(corpo)[:maxCorpoNoPrompt]) + "…"
		}
		fmt.Fprintf(&sb, "\n[%d] %s", i+1, p.Titulo)
		if p.Categoria != "" {
			fmt.Fprintf(&sb, " (%s)", p.Categoria)
		}
		fmt.Fprintf(&sb, "\n%s\n", corpo)
	}

	out := []mensagemLLM{{Role: "system", Content: sb.String()}}
	if len(msgs) > mensagensEnviadasAoLLM {
		msgs = msgs[len(msgs)-mensagensEnviadasAoLLM:]
	}
	for i, m := range msgs {
		role, texto := "user", m.Texto
		if m.Papel == "assistente" {
			role = "assistant"
		}
		// O Qwen3 "pensa" antes de responder, e isso gasta neurons; /no_think desliga
		if i == len(msgs)-1 {
			texto += " /no_think"
		}
		out = append(out, mensagemLLM{Role: role, Content: texto})
	}
	return out
}

type usoTokens struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// chamarWorkersAI usa o endpoint compatível com OpenAI da Workers AI
func (h *AssistenteHandler) chamarWorkersAI(ctx context.Context, msgs []mensagemLLM) (string, usoTokens, error) {
	corpo, err := json.Marshal(map[string]any{
		"model":       h.cfg.CFAIModel,
		"messages":    msgs,
		"max_tokens":  maxTokensResposta,
		"temperature": 0.3,
	})
	if err != nil {
		return "", usoTokens{}, err
	}

	url := fmt.Sprintf("%s/accounts/%s/ai/v1/chat/completions", h.cfg.CFAPIBaseURL, h.cfg.CFAccountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(corpo))
	if err != nil {
		return "", usoTokens{}, err
	}
	req.Header.Set("Authorization", "Bearer "+h.cfg.CFAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.http.Do(req)
	if err != nil {
		return "", usoTokens{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage usoTokens `json:"usage"`
	}
	if resp.StatusCode != http.StatusOK {
		var detalhe bytes.Buffer
		_, _ = detalhe.ReadFrom(io.LimitReader(resp.Body, 2<<10))
		return "", usoTokens{}, fmt.Errorf("workers ai respondeu %d: %s", resp.StatusCode, detalhe.String())
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", usoTokens{}, fmt.Errorf("resposta da workers ai inválida: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", out.Usage, errors.New("workers ai não retornou resposta")
	}
	return out.Choices[0].Message.Content, out.Usage, nil
}

var rePensamento = regexp.MustCompile(`(?s)<think>.*?</think>`)

// limparPensamento remove o bloco <think> que o Qwen3 às vezes devolve vazio
func limparPensamento(s string) string {
	s = rePensamento.ReplaceAllString(s, "")
	if i := strings.Index(s, "</think>"); i >= 0 {
		s = s[i+len("</think>"):]
	}
	return strings.TrimSpace(s)
}

var (
	reFontes = regexp.MustCompile(`(?im)^\s*\**FONTES:?\**:?\s*(.*)$`)
	reNumero = regexp.MustCompile(`\d+`)
)

// separarFontes tira a linha "FONTES: 1, 3" da resposta e devolve os números
func separarFontes(s string) (string, []int) {
	locs := reFontes.FindAllStringSubmatchIndex(s, -1)
	if len(locs) == 0 {
		return strings.TrimSpace(s), nil
	}
	ultima := locs[len(locs)-1]
	lista := s[ultima[2]:ultima[3]]
	texto := strings.TrimSpace(s[:ultima[0]] + s[ultima[1]:])

	var indices []int
	vistos := map[int]bool{}
	for _, parte := range reNumero.FindAllString(lista, -1) {
		if n, err := strconv.Atoi(parte); err == nil && !vistos[n] {
			vistos[n] = true
			indices = append(indices, n)
		}
	}
	return texto, indices
}

var reTitulo = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)

// extrairContato devolve, sem alterar, a seção "## Contato" do procedimento.
// É o que aparece no cartão do chat, então telefone e ramal nunca passam
// pelo modelo.
func extrairContato(corpo string) *string {
	linhas := strings.Split(strings.ReplaceAll(corpo, "\r\n", "\n"), "\n")
	nivel := 0
	var secao []string
	for _, l := range linhas {
		if m := reTitulo.FindStringSubmatch(l); m != nil {
			if nivel > 0 && len(m[1]) <= nivel {
				break
			}
			if nivel == 0 {
				t := strings.ToLower(strings.TrimSpace(m[2]))
				if t == "contato" || t == "contatos" {
					nivel = len(m[1])
				}
				continue
			}
		}
		if nivel > 0 {
			secao = append(secao, l)
		}
	}
	texto := strings.TrimSpace(strings.Join(secao, "\n"))
	if texto == "" {
		return nil
	}
	return &texto
}
