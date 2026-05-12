package demos

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunLists executa a demo de Lists em 4 partes:
//  1. RPUSH / LRANGE / LLEN — montar e inspecionar uma fila
//  2. LPOP / RPOP           — consumir elementos das extremidades
//  3. LPUSH + LRANGE        — usar como pilha (LIFO)
//  4. BLPOP                 — pop bloqueante simulando worker aguardando job
func RunLists(ctx context.Context, rdb *redis.Client) {
	header("Lists & Filas")

	part1BuildQueue(ctx, rdb)
	pause()

	part2PopElements(ctx, rdb)
	pause()

	part3Stack(ctx, rdb)
	pause()

	part4BlockingPop(ctx, rdb)

	fmt.Println("\n✅  Demo de Lists concluída.")
}

// ── Parte 1: RPUSH / LRANGE / LLEN ───────────────────────────────────────────

func part1BuildQueue(ctx context.Context, rdb *redis.Client) {
	section("1/4 — RPUSH, LRANGE e LLEN: montando uma fila")

	explain("List é uma lista encadeada ordenada pela posição de inserção.")
	explain("RPUSH insere no fim (tail) — comportamento natural de fila FIFO.")
	explain("LPUSH insere no início (head) — usado em pilhas ou filas inversas.")
	fmt.Println()

	key := "tour:fila:emails"

	// Limpa eventual sobra de execução anterior
	rdb.Del(ctx, key)

	jobs := []string{
		`{"para":"ana@example.com",   "assunto":"Boas-vindas"}`,
		`{"para":"bruno@example.com", "assunto":"Confirmação de pedido"}`,
		`{"para":"carla@example.com", "assunto":"Resetar senha"}`,
		`{"para":"diego@example.com", "assunto":"Newsletter"}`,
		`{"para":"eva@example.com",   "assunto":"Fatura do mês"}`,
	}

	explain("Enfileirando 5 e-mails para envio:")
	fmt.Println()

	for _, job := range jobs {
		run(fmt.Sprintf("RPUSH %s '%s'", key, job))
		n, _ := rdb.RPush(ctx, key, job).Result()
		result(fmt.Sprintf("(integer) %d  ← tamanho atual da fila", n))
		time.Sleep(80 * time.Millisecond)
	}

	fmt.Println()

	// LLEN
	run(fmt.Sprintf("LLEN %s", key))
	llen, _ := rdb.LLen(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d", llen))

	fmt.Println()

	// LRANGE completo
	run(fmt.Sprintf("LRANGE %s 0 -1  ← 0 = início, -1 = fim (toda a lista)", key))
	items, _ := rdb.LRange(ctx, key, 0, -1).Result()
	for i, v := range items {
		result(fmt.Sprintf("%d) %s", i+1, v))
	}

	fmt.Println()

	// LRANGE parcial — só os 2 primeiros
	run(fmt.Sprintf("LRANGE %s 0 1  ← só os 2 primeiros (próximos a serem consumidos)", key))
	items, _ = rdb.LRange(ctx, key, 0, 1).Result()
	for i, v := range items {
		result(fmt.Sprintf("%d) %s", i+1, v))
	}

	// LINDEX — elemento por posição
	fmt.Println()
	run(fmt.Sprintf("LINDEX %s 2  ← elemento na posição 2 (zero-based)", key))
	item, _ := rdb.LIndex(ctx, key, 2).Result()
	result(fmt.Sprintf("%q", item))

	explain("\n💡 LRANGE não remove os elementos — é só uma leitura. Use POP para consumir.")
}

// ── Parte 2: LPOP / RPOP ─────────────────────────────────────────────────────

func part2PopElements(ctx context.Context, rdb *redis.Client) {
	section("2/4 — LPOP e RPOP: consumindo a fila")

	explain("LPOP remove e retorna o elemento do início (head) — padrão FIFO.")
	explain("RPOP remove e retorna o elemento do fim (tail) — padrão LIFO/pilha.")
	fmt.Println()

	key := "tour:fila:emails"

	// Garante que a fila tem itens
	llen, _ := rdb.LLen(ctx, key).Result()
	if llen == 0 {
		explain("(fila vazia — recriando para a demo)")
		rdb.RPush(ctx, key,
			`{"para":"ana@example.com",   "assunto":"Boas-vindas"}`,
			`{"para":"bruno@example.com", "assunto":"Confirmação de pedido"}`,
			`{"para":"carla@example.com", "assunto":"Resetar senha"}`,
			`{"para":"diego@example.com", "assunto":"Newsletter"}`,
			`{"para":"eva@example.com",   "assunto":"Fatura do mês"}`,
		)
		fmt.Println()
	}

	explain("Consumindo a fila com LPOP (ordem FIFO — primeiro que entrou, primeiro que sai):")
	fmt.Println()

	for {
		run(fmt.Sprintf("LPOP %s", key))
		val, err := rdb.LPop(ctx, key).Result()
		if err == redis.Nil {
			result("(nil)  ← fila vazia")
			break
		}
		llen, _ := rdb.LLen(ctx, key).Result()
		result(fmt.Sprintf("%s", val))
		explain(fmt.Sprintf("   restam %d itens na fila", llen))
		fmt.Println()
		time.Sleep(150 * time.Millisecond)
	}

	// Reabastece para mostrar RPOP
	fmt.Println()
	explain("Reabastecendo a fila e consumindo com RPOP (ordem inversa — LIFO):")
	fmt.Println()

	rdb.RPush(ctx, key, "job-A", "job-B", "job-C")

	run(fmt.Sprintf("LRANGE %s 0 -1  ← estado atual", key))
	items, _ := rdb.LRange(ctx, key, 0, -1).Result()
	for i, v := range items {
		result(fmt.Sprintf("%d) %q", i+1, v))
	}
	fmt.Println()

	for {
		run(fmt.Sprintf("RPOP %s", key))
		val, err := rdb.RPop(ctx, key).Result()
		if err == redis.Nil {
			result("(nil)  ← fila vazia")
			break
		}
		result(fmt.Sprintf("%q", val))
		time.Sleep(150 * time.Millisecond)
	}

	explain("\n💡 FIFO (fila): RPUSH para enfileirar + LPOP para consumir.")
	explain("   LIFO (pilha): LPUSH para empilhar + LPOP para desempilhar.")
}

// ── Parte 3: Pilha (LIFO) ────────────────────────────────────────────────────

func part3Stack(ctx context.Context, rdb *redis.Client) {
	section("3/4 — LPUSH + LPOP: usando List como pilha (LIFO)")

	explain("Navegador, editor de texto, undo/redo — todos usam pilha.")
	explain("Com LPUSH + LPOP a List se comporta exatamente como uma pilha.")
	fmt.Println()

	key := "tour:pilha:historico"
	rdb.Del(ctx, key)

	pages := []string{
		"home",
		"produtos",
		"produto/42",
		"carrinho",
		"checkout",
	}

	explain("Simulando histórico de navegação (cada página visitada vai para o topo):")
	fmt.Println()

	for _, page := range pages {
		run(fmt.Sprintf("LPUSH %s \"%s\"", key, page))
		n, _ := rdb.LPush(ctx, key, page).Result()
		result(fmt.Sprintf("(integer) %d", n))
		time.Sleep(80 * time.Millisecond)
	}

	fmt.Println()
	run(fmt.Sprintf("LRANGE %s 0 -1  ← topo = índice 0 (última página visitada)", key))
	items, _ := rdb.LRange(ctx, key, 0, -1).Result()
	for i, v := range items {
		if i == 0 {
			result(fmt.Sprintf("%d) %q  ← página atual", i+1, v))
		} else {
			result(fmt.Sprintf("%d) %q", i+1, v))
		}
	}

	fmt.Println()
	explain("Voltando nas páginas (botão ← do navegador):")
	fmt.Println()

	for i := 0; i < 3; i++ {
		run(fmt.Sprintf("LPOP %s", key))
		val, _ := rdb.LPop(ctx, key).Result()
		result(fmt.Sprintf("%q  ← saindo desta página", val))

		topo, _ := rdb.LIndex(ctx, key, 0).Result()
		explain(fmt.Sprintf("   página atual agora: %q", topo))
		fmt.Println()
		time.Sleep(150 * time.Millisecond)
	}

	explain("💡 LPUSH + LPOP = pilha. RPUSH + LPOP = fila. A List suporta os dois padrões.")
}

// ── Parte 4: BLPOP ───────────────────────────────────────────────────────────

func part4BlockingPop(ctx context.Context, rdb *redis.Client) {
	section("4/4 — BLPOP: worker bloqueante aguardando jobs")

	explain("BLPOP é como LPOP, mas bloqueia a conexão até um item chegar na fila.")
	explain("Ideal para workers que ficam aguardando tarefas sem fazer polling.")
	explain("O segundo argumento é o timeout em segundos (0 = espera para sempre).")
	fmt.Println()

	key := "tour:fila:jobs"
	rdb.Del(ctx, key)

	explain("Cenário: worker está esperando. Vamos publicar 3 jobs com intervalo.")
	fmt.Println()

	// Publica jobs numa goroutine com delay para simular produtor externo
	go func() {
		jobsToSend := []string{"job:resize-image", "job:send-email", "job:generate-report"}
		for i, job := range jobsToSend {
			time.Sleep(time.Duration(i+1) * 800 * time.Millisecond)
			rdb.RPush(ctx, key, job)
		}
	}()

	// Worker consome com BLPOP
	for i := 0; i < 3; i++ {
		run(fmt.Sprintf("BLPOP %s 5  ← aguardando (timeout: 5s)...", key))
		res, err := rdb.BLPop(ctx, 5*time.Second, key).Result()
		if err != nil {
			result("(nil)  ← timeout — nenhum item chegou")
			break
		}
		result(fmt.Sprintf("1) %q  ← nome da fila", res[0]))
		result(fmt.Sprintf("2) %q  ← job recebido ✅", res[1]))
		explain(fmt.Sprintf("   worker processando: %s", res[1]))
		fmt.Println()
	}

	// Fila vazia — mostra timeout
	fmt.Println()
	explain("Tentando consumir de uma fila vazia com timeout de 2s:")
	fmt.Println()

	run(fmt.Sprintf("BLPOP %s 2  ← timeout em 2s", key))
	_, err := rdb.BLPop(ctx, 2*time.Second, key).Result()
	if err == redis.Nil {
		result("(nil)  ← timeout atingido, nenhum item na fila")
	}

	explain("\n💡 Em produção, use BLPOP com timeout > 0 e recomece o loop após cada retorno.")
	explain("   Para filas com múltiplos consumidores e ack, considere Redis Streams (demo 8).")
}
