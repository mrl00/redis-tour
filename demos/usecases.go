package demos

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ── [6] Cache com TTL ─────────────────────────────────────────────────────────

// RunCache demonstra o padrão cache-aside:
//  1. Cache miss  — busca no "banco", armazena no Redis com TTL
//  2. Cache hit   — retorna do Redis sem tocar no banco
//  3. Expiração   — TTL chega a zero, próxima leitura vai ao banco novamente
func RunCache(ctx context.Context, rdb *redis.Client) {
	header("Cache com TTL — padrão cache-aside")

	part1CacheMissHit(ctx, rdb)
	pause()

	part2Expiration(ctx, rdb)

	fmt.Println("\n✅  Demo de Cache concluída.")
}

type Produto struct {
	ID    int     `json:"id"`
	Nome  string  `json:"nome"`
	Preco float64 `json:"preco"`
	Stock int     `json:"estoque"`
}

// fetchDoBanco simula uma query lenta num banco relacional.
func fetchDoBanco(id int) Produto {
	time.Sleep(250 * time.Millisecond) // simula latência de I/O
	return Produto{
		ID:    id,
		Nome:  fmt.Sprintf("Produto #%d", id),
		Preco: 199.90,
		Stock: 42,
	}
}

func cacheKey(id int) string {
	return fmt.Sprintf("tour:cache:produto:%d", id)
}

func part1CacheMissHit(ctx context.Context, rdb *redis.Client) {
	section("1/2 — Cache miss e cache hit")

	explain("Padrão cache-aside (lazy loading):")
	explain("  1. Tenta ler do Redis")
	explain("  2. Se não existe (miss) → busca no banco, salva no Redis")
	explain("  3. Se existe (hit) → retorna direto do Redis")
	fmt.Println()

	prodID := 42
	key := cacheKey(prodID)
	ttl := 10 * time.Second

	rdb.Del(ctx, key)

	// ── Primeira requisição: MISS ──
	explain("── Requisição 1 (cache miss) ──")
	fmt.Println()

	run(fmt.Sprintf("GET %s", key))
	val, err := rdb.Get(ctx, key).Result()

	if err == redis.Nil {
		result("(nil)  ← não está no cache")
		fmt.Println()

		explain("   indo ao banco de dados...")
		start := time.Now()
		produto := fetchDoBanco(prodID)
		dbLatency := time.Since(start)
		explain(fmt.Sprintf("   ✅  dado obtido do banco em %s", dbLatency.Round(time.Millisecond)))

		payload, _ := json.Marshal(produto)

		fmt.Println()
		run(fmt.Sprintf("SET %s '%s' EX %d", key, string(payload), int(ttl.Seconds())))
		rdb.Set(ctx, key, payload, ttl)
		result(fmt.Sprintf("+OK  ← armazenado no cache por %s", ttl))

		fmt.Println()
		explain(fmt.Sprintf("   Resposta: %s", string(payload)))
	} else {
		explain(fmt.Sprintf("   (cache já populado): %s", val))
	}

	fmt.Println()

	// ── Segunda requisição: HIT ──
	explain("── Requisição 2 (cache hit) ──")
	fmt.Println()

	run(fmt.Sprintf("GET %s", key))
	start := time.Now()
	val, err = rdb.Get(ctx, key).Result()
	redisLatency := time.Since(start)

	if err == nil {
		result(fmt.Sprintf("%s", val))
		fmt.Println()
		explain(fmt.Sprintf("   ✅  dado obtido do cache em %s  (era ~250ms no banco)", redisLatency.Round(time.Microsecond)))

		run(fmt.Sprintf("TTL %s", key))
		ttlLeft, _ := rdb.TTL(ctx, key).Result()
		result(fmt.Sprintf("(integer) %d  ← segundos até expirar", int(ttlLeft.Seconds())))
	}

	// ── Terceira requisição: HIT ──
	fmt.Println()
	explain("── Requisição 3 (cache hit novamente) ──")
	fmt.Println()

	run(fmt.Sprintf("GET %s", key))
	start = time.Now()
	val, _ = rdb.Get(ctx, key).Result()
	redisLatency = time.Since(start)
	result(fmt.Sprintf("%s", val))
	explain(fmt.Sprintf("\n   ✅  %s  — Redis não acessa disco nem rede externa", redisLatency.Round(time.Microsecond)))
}

func part2Expiration(ctx context.Context, rdb *redis.Client) {
	section("2/2 — Expiração e cache invalidation")

	explain("Quando o TTL chega a zero o Redis remove a chave automaticamente.")
	explain("A próxima requisição encontra um miss e busca dados frescos no banco.")
	fmt.Println()

	prodID := 99
	key := cacheKey(prodID)
	ttl := 3 * time.Second

	// Popula o cache com TTL curto
	produto := Produto{ID: prodID, Nome: "Produto #99", Preco: 59.90, Stock: 7}
	payload, _ := json.Marshal(produto)

	run(fmt.Sprintf("SET %s '<json>' EX %d  ← TTL curto para a demo", key, int(ttl.Seconds())))
	rdb.Set(ctx, key, payload, ttl)
	result("+OK")

	fmt.Println()
	explain(fmt.Sprintf("Monitorando o TTL por %d segundos...", int(ttl.Seconds())+1))
	fmt.Println()

	for i := 0; i <= int(ttl.Seconds())+1; i++ {
		ttlLeft, _ := rdb.TTL(ctx, key).Result()
		exists, _ := rdb.Exists(ctx, key).Result()

		run(fmt.Sprintf("TTL %s  (t+%ds)", key, i))

		if exists == 0 {
			result("(integer) -2  ← chave expirou!")
			fmt.Println()

			explain("   próxima requisição vai ao banco (miss)...")
			start := time.Now()
			fresh := fetchDoBanco(prodID)
			dbLatency := time.Since(start)
			freshPayload, _ := json.Marshal(fresh)

			run(fmt.Sprintf("SET %s '<json>' EX %d  ← repovoando cache", key, int(ttl.Seconds())))
			rdb.Set(ctx, key, freshPayload, ttl)
			result(fmt.Sprintf("+OK  ← cache repovoado em %s", dbLatency.Round(time.Millisecond)))
			break
		}

		result(fmt.Sprintf("(integer) %d", int(ttlLeft.Seconds())))
		time.Sleep(1 * time.Second)
	}

	fmt.Println()
	explain("💡 Estratégias de invalidação:")
	explain("   • TTL fixo (usado aqui) — simples, aceita dados levemente desatualizados")
	explain("   • DEL explícito         — invalida imediatamente ao salvar no banco")
	explain("   • Cache-aside + write-through — grava no cache junto com o banco")
}

// ── [7] Rate Limiter ──────────────────────────────────────────────────────────

// RunRateLimit demonstra duas estratégias de rate limiting com Redis:
//  1. Fixed window  — INCR + EXPIRE numa janela fixa de tempo
//  2. Sliding window — ZADD + ZREMRANGEBYSCORE com timestamps
func RunRateLimit(ctx context.Context, rdb *redis.Client) {
	header("Rate Limiter")

	part1FixedWindow(ctx, rdb)
	pause()

	part2SlidingWindow(ctx, rdb)

	fmt.Println("\n✅  Demo de Rate Limiter concluída.")
}

func part1FixedWindow(ctx context.Context, rdb *redis.Client) {
	section("1/2 — Fixed window: INCR + EXPIRE")

	explain("A estratégia mais simples: conta requisições numa janela fixa de tempo.")
	explain("Quando o contador chega ao limite, a requisição é bloqueada.")
	explain("A janela reseta automaticamente quando o TTL expira.")
	fmt.Println()

	const (
		limite   = 5
		janela   = 10 * time.Second
		totalReq = 8
	)

	ip := "192.168.1.100"
	key := fmt.Sprintf("tour:ratelimit:fixed:%s", ip)

	rdb.Del(ctx, key)

	explain(fmt.Sprintf("Configuração: %d req / %s  —  IP: %s", limite, janela, ip))
	fmt.Println()

	for i := 1; i <= totalReq; i++ {
		count, _ := rdb.Incr(ctx, key).Result()

		// Define o TTL apenas na primeira requisição da janela
		if count == 1 {
			rdb.Expire(ctx, key, janela)
		}

		ttlLeft, _ := rdb.TTL(ctx, key).Result()

		run(fmt.Sprintf("INCR %s  (req #%d)", key, i))

		if count <= limite {
			result(fmt.Sprintf("(integer) %d  ← %d/%d  ✅ permitida  (janela fecha em %ds)",
				count, count, limite, int(ttlLeft.Seconds())))
		} else {
			result(fmt.Sprintf("(integer) %d  ← %d/%d  🚫 BLOQUEADA  (tente em %ds)",
				count, count, limite, int(ttlLeft.Seconds())))
		}

		time.Sleep(120 * time.Millisecond)
	}

	fmt.Println()
	explain("💡 Problema da fixed window: rajada no limite da janela.")
	explain("   5 req no final de uma janela + 5 no início da próxima = 10 req em 1s.")
	explain("   Para evitar isso, use sliding window (parte 2).")
}

func part2SlidingWindow(ctx context.Context, rdb *redis.Client) {
	section("2/2 — Sliding window: ZADD + ZREMRANGEBYSCORE")

	explain("Conta requisições num intervalo móvel — sempre os últimos N segundos.")
	explain("Cada requisição é registrada como membro no Sorted Set com seu timestamp.")
	explain("Antes de contar, remove entradas fora da janela. Sem problema de rajada.")
	fmt.Println()

	const (
		limite   = 5
		janelaSW = 10 * time.Second
		totalReq = 8
	)

	ip := "192.168.1.200"
	key := fmt.Sprintf("tour:ratelimit:sliding:%s", ip)

	rdb.Del(ctx, key)

	explain(fmt.Sprintf("Configuração: %d req / %s  —  IP: %s", limite, janelaSW, ip))
	fmt.Println()

	for i := 1; i <= totalReq; i++ {
		now := time.Now()
		windowStart := float64(now.Add(-janelaSW).UnixMilli())
		nowMs := float64(now.UnixMilli())

		// Remove entradas fora da janela
		rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%.0f", windowStart))

		// Conta quantas requisições estão na janela
		count, _ := rdb.ZCard(ctx, key).Result()

		run(fmt.Sprintf("ZREMRANGEBYSCORE %s -inf %.0f  (limpa fora da janela)", key, windowStart))
		run(fmt.Sprintf("ZCARD %s  → %d req nos últimos %s", key, count, janelaSW))

		if count < int64(limite) {
			member := fmt.Sprintf("req:%d:%d", i, now.UnixNano())
			rdb.ZAdd(ctx, key, redis.Z{Score: nowMs, Member: member})
			rdb.Expire(ctx, key, janelaSW)

			run(fmt.Sprintf("ZADD %s %.0f \"%s\"", key, nowMs, member))
			result(fmt.Sprintf("(integer) 1  ← req #%d  %d/%d  ✅ permitida", i, count+1, limite))
		} else {
			oldest, _ := rdb.ZRangeWithScores(ctx, key, 0, 0).Result()
			var retryMs int64
			if len(oldest) > 0 {
				retryMs = int64(oldest[0].Score) + janelaSW.Milliseconds() - now.UnixMilli()
			}
			result(fmt.Sprintf("← req #%d  %d/%d  🚫 BLOQUEADA  (tente em ~%dms)", i, count, limite, retryMs))
		}

		fmt.Println()
		time.Sleep(150 * time.Millisecond)
	}

	explain("💡 Sliding window é mais precisa mas usa mais memória (1 entrada por requisição).")
	explain("   Para alto volume, use fixed window ou token bucket com scripts Lua atômicos.")
}

// ── [8] Session Store ─────────────────────────────────────────────────────────

// RunSessionStore demonstra o ciclo completo de uma sessão de usuário:
//  1. Login      — cria sessão com token UUID via HSET + EXPIRE
//  2. Requisições — lê e valida sessão com HGETALL + TTL
//  3. Renovação  — estende o TTL com EXPIRE a cada requisição autenticada
//  4. Logout     — destrói a sessão com DEL
func RunSessionStore(ctx context.Context, rdb *redis.Client) {
	header("Session Store")

	part1Login(ctx, rdb)
	pause()

	part2AuthRequests(ctx, rdb)
	pause()

	part3Renewal(ctx, rdb)
	pause()

	part4Logout(ctx, rdb)

	fmt.Println("\n✅  Demo de Session Store concluída.")
}

// token simula a geração de um UUID v4 sem dependência externa.
func generateToken() string {
	now := time.Now().UnixNano()
	return fmt.Sprintf("%08x-%04x-4%03x-%04x-%012x",
		now&0xffffffff,
		(now>>32)&0xffff,
		(now>>48)&0x0fff,
		0x8000|((now>>16)&0x3fff),
		now&0xffffffffffff,
	)
}

func sessionKey(token string) string {
	return fmt.Sprintf("tour:session:%s", token)
}

var activeToken string // compartilhado entre as partes da demo

func part1Login(ctx context.Context, rdb *redis.Client) {
	section("1/4 — Login: criando a sessão")

	explain("Ao autenticar, o servidor gera um token opaco (UUID) e armazena")
	explain("os dados da sessão num Hash com TTL de inatividade.")
	explain("O token é enviado ao cliente via cookie — o Redis fica com os dados.")
	fmt.Println()

	token := generateToken()
	activeToken = token
	key := sessionKey(token)
	ttl := 30 * time.Second

	explain("Usuário enviou credenciais corretas. Criando sessão...")
	fmt.Println()

	run(fmt.Sprintf("HSET %s \\", key))
	run("    user_id  \"42\"")
	run("    nome     \"Ana Lima\"")
	run("    email    \"ana@example.com\"")
	run("    role     \"admin\"")
	run("    ip       \"203.0.113.10\"")
	run(fmt.Sprintf("    criada_em \"%s\"", time.Now().Format("2006-01-02 15:04:05")))

	n, _ := rdb.HSet(ctx, key, map[string]any{
		"user_id":   "42",
		"nome":      "Ana Lima",
		"email":     "ana@example.com",
		"role":      "admin",
		"ip":        "203.0.113.10",
		"criada_em": time.Now().Format("2006-01-02 15:04:05"),
	}).Result()
	result(fmt.Sprintf("(integer) %d  ← campos criados", n))

	fmt.Println()
	run(fmt.Sprintf("EXPIRE %s %d  ← sessão expira em %s de inatividade", key, int(ttl.Seconds()), ttl))
	rdb.Expire(ctx, key, ttl)
	result(fmt.Sprintf("(integer) 1"))

	fmt.Println()
	explain(fmt.Sprintf("✅  Sessão criada. Token enviado ao cliente:"))
	explain(fmt.Sprintf("   %s", token))

	fmt.Println()
	explain("💡 O token nunca expõe dados do usuário — é apenas uma chave aleatória.")
	explain("   Os dados ficam no servidor (Redis). O cliente só guarda o token no cookie.")
}

func part2AuthRequests(ctx context.Context, rdb *redis.Client) {
	section("2/4 — Requisições autenticadas: lendo a sessão")

	explain("A cada requisição, o servidor recebe o token do cookie,")
	explain("busca a sessão no Redis e valida se ainda é válida.")
	fmt.Println()

	token := activeToken
	key := sessionKey(token)

	// Simula 3 requisições autenticadas
	endpoints := []string{
		"GET /api/perfil",
		"GET /api/pedidos",
		"POST /api/configuracoes",
	}

	for i, endpoint := range endpoints {
		explain(fmt.Sprintf("── Requisição %d: %s ──", i+1, endpoint))
		fmt.Println()

		// Verifica se a sessão existe
		run(fmt.Sprintf("EXISTS %s", key))
		exists, _ := rdb.Exists(ctx, key).Result()
		result(fmt.Sprintf("(integer) %d", exists))

		if exists == 0 {
			result("← sessão não encontrada — redirecionar para login 🔒")
			continue
		}

		// Lê os dados da sessão
		run(fmt.Sprintf("HGETALL %s", key))
		fields, _ := rdb.HGetAll(ctx, key).Result()
		j := 1
		for campo, valor := range fields {
			result(fmt.Sprintf("%2d) %-12s  %s", j, campo, valor))
			j++
		}

		// Verifica a role para autorização
		fmt.Println()
		run(fmt.Sprintf("HGET %s role  ← verifica permissão", key))
		role, _ := rdb.HGet(ctx, key, "role").Result()
		result(fmt.Sprintf("%q  ← acesso autorizado ✅", role))

		ttlLeft, _ := rdb.TTL(ctx, key).Result()
		explain(fmt.Sprintf("\n   sessão válida por mais %ds", int(ttlLeft.Seconds())))
		fmt.Println()

		time.Sleep(200 * time.Millisecond)
	}
}

func part3Renewal(ctx context.Context, rdb *redis.Client) {
	section("3/4 — Renovação: sliding expiration")

	explain("A sessão renova o TTL a cada requisição autenticada (sliding expiration).")
	explain("Isso mantém usuários ativos logados sem precisar autenticar de novo.")
	explain("Usuários inativos são deslogados automaticamente quando o TTL expira.")
	fmt.Println()

	token := activeToken
	key := sessionKey(token)
	ttl := 30 * time.Second

	run(fmt.Sprintf("TTL %s  ← TTL antes da renovação", key))
	before, _ := rdb.TTL(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d", int(before.Seconds())))

	explain("\n   usuário fez uma requisição — renovando sessão...")
	time.Sleep(500 * time.Millisecond)
	fmt.Println()

	run(fmt.Sprintf("EXPIRE %s %d  ← renova para mais %s", key, int(ttl.Seconds()), ttl))
	rdb.Expire(ctx, key, ttl)
	result("(integer) 1")

	run(fmt.Sprintf("TTL %s  ← TTL após renovação", key))
	after, _ := rdb.TTL(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d  ← voltou para %s ✅", int(after.Seconds()), ttl))

	fmt.Println()
	explain("Comparação de estratégias de expiração de sessão:")
	fmt.Println()
	explain("   Absolute expiration  — TTL fixo desde a criação, não renova")
	explain("                          bom para tokens de API com validade garantida")
	fmt.Println()
	explain("   Sliding expiration   — TTL renova a cada requisição (usado aqui)")
	explain("                          bom para sessões web — deslogado apenas por inatividade")
}

func part4Logout(ctx context.Context, rdb *redis.Client) {
	section("4/4 — Logout: destruindo a sessão")

	explain("Logout remove a sessão do Redis imediatamente.")
	explain("Mesmo que alguém capture o token, ele não terá mais dados associados.")
	fmt.Println()

	token := activeToken
	key := sessionKey(token)

	// Estado antes do logout
	run(fmt.Sprintf("EXISTS %s  ← sessão existe?", key))
	exists, _ := rdb.Exists(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d  ← sim ✅", exists))

	run(fmt.Sprintf("HGET %s nome", key))
	nome, _ := rdb.HGet(ctx, key, "nome").Result()
	result(fmt.Sprintf("%q", nome))

	fmt.Println()
	explain("Usuário clicou em \"Sair\"...")
	fmt.Println()

	// DEL — remove a sessão
	run(fmt.Sprintf("DEL %s", key))
	deleted, _ := rdb.Del(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d  ← sessão removida 🗑️", deleted))

	fmt.Println()

	// Tenta usar o token após logout
	explain("Próxima requisição com o mesmo token (cookie ainda no browser):")
	fmt.Println()

	run(fmt.Sprintf("EXISTS %s", key))
	exists, _ = rdb.Exists(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d  ← sessão não existe", exists))

	run(fmt.Sprintf("HGETALL %s", key))
	fields, _ := rdb.HGetAll(ctx, key).Result()
	if len(fields) == 0 {
		result("(empty array)  ← redirecionar para login 🔒")
	}

	fmt.Println()
	explain("💡 Em múltiplos servidores (cluster), todos leem o mesmo Redis.")
	explain("   Logout num servidor invalida a sessão para todos — impossível com cookies locais.")
}
