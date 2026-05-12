package demos

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunStrings executa a demo de Strings em 4 partes:
//  1. SET / GET básico
//  2. INCR simulando contador de visitas
//  3. SET com EX mostrando TTL diminuindo
//  4. SETNX simulando lock distribuído
func RunStrings(ctx context.Context, rdb *redis.Client) {
	header("Strings & Contadores")

	part1BasicSetGet(ctx, rdb)
	pause()

	part2IncrCounter(ctx, rdb)
	pause()

	part3TTL(ctx, rdb)
	pause()

	part4DistributedLock(ctx, rdb)

	fmt.Println("\n✅  Demo de Strings concluída.")
}

// ── Parte 1: SET / GET ────────────────────────────────────────────────────────

func part1BasicSetGet(ctx context.Context, rdb *redis.Client) {
	section("1/4 — SET e GET básico")

	explain("O comando SET armazena qualquer valor (string, número, JSON) numa chave.")
	explain("O comando GET recupera o valor. Se a chave não existir, retorna nil.")
	fmt.Println()

	key := "tour:greeting"
	value := "Olá, Redis!"

	run(fmt.Sprintf("SET %s \"%s\"", key, value))
	rdb.Set(ctx, key, value, 0)
	ok()

	run(fmt.Sprintf("GET %s", key))
	val, _ := rdb.Get(ctx, key).Result()
	result(fmt.Sprintf("%q", val))

	// Sobrescreve o valor — SET não falha se a chave já existir
	newValue := "Valor atualizado"
	run(fmt.Sprintf("SET %s \"%s\"  ← sobrescrevendo", key, newValue))
	rdb.Set(ctx, key, newValue, 0)
	ok()

	run(fmt.Sprintf("GET %s", key))
	val, _ = rdb.Get(ctx, key).Result()
	result(fmt.Sprintf("%q", val))

	// Chave inexistente
	run("GET tour:nao_existe")
	res, err := rdb.Get(ctx, "tour:nao_existe").Result()
	if err == redis.Nil {
		result("(nil)")
	} else {
		result(res)
	}

	explain("\n💡 SET sempre sobrescreve. Use SETNX (parte 4) para gravar só se não existir.")
}

// ── Parte 2: INCR ─────────────────────────────────────────────────────────────

func part2IncrCounter(ctx context.Context, rdb *redis.Client) {
	section("2/4 — INCR: contador atômico")

	explain("INCR incrementa um inteiro atomicamente — sem race condition, mesmo com")
	explain("múltiplas goroutines batendo na mesma chave ao mesmo tempo.")
	fmt.Println()

	key := "tour:visitas"

	// Garante que começa do zero
	rdb.Del(ctx, key)

	run(fmt.Sprintf("# Simulando 5 requisições chegando na chave \"%s\"", key))
	fmt.Println()

	for i := 1; i <= 5; i++ {
		run(fmt.Sprintf("INCR %s", key))
		n, _ := rdb.Incr(ctx, key).Result()
		result(fmt.Sprintf("(integer) %d", n))
		time.Sleep(80 * time.Millisecond)
	}

	explain("\n💡 INCR cria a chave com valor 1 se ela não existir — não precisa de SET antes.")

	// Mostra INCRBY
	fmt.Println()
	run(fmt.Sprintf("INCRBY %s 10  ← incrementa de 10 em 10", key))
	n, _ := rdb.IncrBy(ctx, key, 10).Result()
	result(fmt.Sprintf("(integer) %d", n))

	run(fmt.Sprintf("DECRBY %s 3   ← decrementa 3", key))
	n, _ = rdb.DecrBy(ctx, key, 3).Result()
	result(fmt.Sprintf("(integer) %d", n))
}

// ── Parte 3: TTL ─────────────────────────────────────────────────────────────

func part3TTL(ctx context.Context, rdb *redis.Client) {
	section("3/4 — SET com EX: expiração automática")

	explain("Toda chave do Redis pode ter um Time To Live (TTL).")
	explain("Quando o TTL chega a zero, o Redis remove a chave automaticamente.")
	fmt.Println()

	key := "tour:cache_produto"
	ttlSec := 5

	run(fmt.Sprintf("SET %s \"{\\\"id\\\":1, \\\"preco\\\":99.90}\" EX %d", key, ttlSec))
	rdb.Set(ctx, key, `{"id":1, "preco":99.90}`, time.Duration(ttlSec)*time.Second)
	ok()

	// Lê o valor e o TTL atual
	run(fmt.Sprintf("GET %s", key))
	val, _ := rdb.Get(ctx, key).Result()
	result(fmt.Sprintf("%q", val))

	fmt.Println()
	explain(fmt.Sprintf("Monitorando TTL por %d segundos...", ttlSec+1))
	fmt.Println()

	for i := 0; i <= ttlSec; i++ {
		ttl, _ := rdb.TTL(ctx, key).Result()
		exists, _ := rdb.Exists(ctx, key).Result()

		if exists == 0 {
			run(fmt.Sprintf("TTL %s  (t+%ds)", key, i))
			result("(integer) -2  ← chave expirou e foi removida!")
			break
		}

		run(fmt.Sprintf("TTL %s  (t+%ds)", key, i))
		result(fmt.Sprintf("(integer) %d", int(ttl.Seconds())))
		time.Sleep(1 * time.Second)
	}

	explain("\n💡 TTL -1 = sem expiração  |  TTL -2 = chave não existe (foi removida)")

	// Mostra PERSIST
	fmt.Println()
	explain("Se quiser remover a expiração de uma chave existente:")
	rdb.Set(ctx, "tour:persistente", "valor", 30*time.Second)
	run("SET tour:persistente \"valor\" EX 30")
	ok()
	run("PERSIST tour:persistente  ← remove o TTL")
	rdb.Persist(ctx, "tour:persistente")
	ok()
	run("TTL tour:persistente")
	ttl, _ := rdb.TTL(ctx, "tour:persistente").Result()
	result(fmt.Sprintf("(integer) %d  ← -1 significa sem expiração", int(ttl.Seconds())))
}

// ── Parte 4: SETNX / Lock ────────────────────────────────────────────────────

func part4DistributedLock(ctx context.Context, rdb *redis.Client) {
	section("4/4 — SETNX: lock distribuído simples")

	explain("SETNX (Set if Not eXists) grava o valor SOMENTE se a chave não existir.")
	explain("Retorna 1 (sucesso) ou 0 (já existia). Ideal para locks e flags únicos.")
	fmt.Println()

	lockKey := "tour:lock:job_cron"
	lockTTL := 3 * time.Second

	// Primeira aquisição — deve funcionar
	run(fmt.Sprintf("SET %s \"worker-1\" NX EX 3", lockKey))
	acquired, _ := rdb.SetNX(ctx, lockKey, "worker-1", lockTTL).Result()
	if acquired {
		result("(integer) 1  ← lock adquirido por worker-1 ✅")
	} else {
		result("(integer) 0  ← lock NÃO adquirido")
	}

	// Segunda tentativa com outro worker — deve falhar
	fmt.Println()
	run(fmt.Sprintf("SET %s \"worker-2\" NX EX 3  ← outro worker tenta", lockKey))
	acquired2, _ := rdb.SetNX(ctx, lockKey, "worker-2", lockTTL).Result()
	if acquired2 {
		result("(integer) 1  ← lock adquirido ✅")
	} else {
		result("(integer) 0  ← lock NÃO adquirido (worker-1 ainda segura) 🚫")
	}

	// Quem tem o lock?
	fmt.Println()
	run(fmt.Sprintf("GET %s  ← quem segura o lock?", lockKey))
	holder, _ := rdb.Get(ctx, lockKey).Result()
	result(fmt.Sprintf("%q", holder))

	// Aguarda o lock expirar
	fmt.Println()
	explain(fmt.Sprintf("Aguardando %s para o lock expirar...", lockTTL))
	time.Sleep(lockTTL + 200*time.Millisecond)

	// Terceira tentativa — agora deve funcionar
	run(fmt.Sprintf("SET %s \"worker-2\" NX EX 3  ← tenta de novo após TTL", lockKey))
	acquired3, _ := rdb.SetNX(ctx, lockKey, "worker-2", lockTTL).Result()
	if acquired3 {
		result("(integer) 1  ← lock adquirido por worker-2 agora ✅")
	} else {
		result("(integer) 0  ← lock NÃO adquirido")
	}

	explain("\n💡 Sempre use EX junto com NX para evitar lock eterno se o worker travar.")
	explain("   Em produção, considere a biblioteca 'redsync' para locks mais robustos.")
}

// ── Helpers de output ────────────────────────────────────────────────────────

func header(title string) {
	line := "═══════════════════════════════════════════"
	fmt.Printf("\n╔%s╗\n", line)
	fmt.Printf("║  %-41s║\n", title)
	fmt.Printf("╚%s╝\n\n", line)
}

func section(title string) {
	fmt.Printf("\n── %s ──────────\n\n", title)
}

func explain(msg string) {
	fmt.Printf("   %s\n", msg)
}

func run(cmd string) {
	fmt.Printf("\033[36m   > %s\033[0m\n", cmd)
}

func result(val string) {
	fmt.Printf("\033[32m   %s\033[0m\n", val)
}

func ok() {
	fmt.Printf("\033[32m   +OK\033[0m\n")
}

func pause() {
	fmt.Println("\n   ··· pressione Enter para continuar ···")
	fmt.Scanln()
}

