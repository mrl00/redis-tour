package demos

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunHashes executa a demo de Hashes em 4 partes:
//  1. HSET / HGET / HGETALL — criar e ler um perfil de usuário
//  2. HMGET — leitura de múltiplos campos de uma vez
//  3. HINCRBY — incrementar campo numérico (sistema de pontos)
//  4. HDEL / HEXISTS — remover campos e verificar existência
func RunHashes(ctx context.Context, rdb *redis.Client) {
	header("Hashes & Objetos")

	part1HsetGetAll(ctx, rdb)
	pause()

	part2Hmget(ctx, rdb)
	pause()

	part3Hincrby(ctx, rdb)
	pause()

	part4HdelHexists(ctx, rdb)

	fmt.Println("\n✅  Demo de Hashes concluída.")
}

// ── Parte 1: HSET / HGET / HGETALL ───────────────────────────────────────────

func part1HsetGetAll(ctx context.Context, rdb *redis.Client) {
	section("1/4 — HSET, HGET e HGETALL")

	explain("Hash é um mapa de campo → valor dentro de uma única chave.")
	explain("Perfeito para representar objetos como usuários, produtos, sessões.")
	explain("Muito mais eficiente do que criar uma chave separada para cada campo.")
	fmt.Println()

	key := "tour:user:1"

	// HSET com múltiplos campos de uma vez (Redis 4.0+)
	run(fmt.Sprintf("HSET %s nome \"Ana Lima\" email \"ana@example.com\" idade 30 cidade \"São Paulo\"", key))
	n, _ := rdb.HSet(ctx, key, map[string]any{
		"nome":   "Ana Lima",
		"email":  "ana@example.com",
		"idade":  30,
		"cidade": "São Paulo",
		"pontos": 0,
		"ativo":  "true",
	}).Result()
	result(fmt.Sprintf("(integer) %d  ← campos criados", n))

	fmt.Println()

	// HGET — campo específico
	run(fmt.Sprintf("HGET %s nome", key))
	val, _ := rdb.HGet(ctx, key, "nome").Result()
	result(fmt.Sprintf("%q", val))

	run(fmt.Sprintf("HGET %s email", key))
	val, _ = rdb.HGet(ctx, key, "email").Result()
	result(fmt.Sprintf("%q", val))

	// Campo que não existe
	run(fmt.Sprintf("HGET %s telefone  ← campo inexistente", key))
	_, err := rdb.HGet(ctx, key, "telefone").Result()
	if err == redis.Nil {
		result("(nil)")
	}

	fmt.Println()

	// HGETALL — retorna todos os campos e valores
	run(fmt.Sprintf("HGETALL %s", key))
	fields, _ := rdb.HGetAll(ctx, key).Result()
	i := 1
	for campo, valor := range fields {
		result(fmt.Sprintf("%d) %q", i, campo))
		result(fmt.Sprintf("%d) %q", i+1, valor))
		i += 2
	}

	fmt.Println()

	// HLEN — número de campos
	run(fmt.Sprintf("HLEN %s  ← quantos campos tem o hash", key))
	hlen, _ := rdb.HLen(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d", hlen))

	// HKEYS / HVALS
	fmt.Println()
	run(fmt.Sprintf("HKEYS %s  ← só os campos", key))
	keys, _ := rdb.HKeys(ctx, key).Result()
	for i, k := range keys {
		result(fmt.Sprintf("%d) %q", i+1, k))
	}

	fmt.Println()
	run(fmt.Sprintf("HVALS %s  ← só os valores", key))
	vals, _ := rdb.HVals(ctx, key).Result()
	for i, v := range vals {
		result(fmt.Sprintf("%d) %q", i+1, v))
	}

	explain("\n💡 HSET cria novos campos ou atualiza existentes. Retorna quantos foram criados (não atualizados).")
}

// ── Parte 2: HMGET ───────────────────────────────────────────────────────────

func part2Hmget(ctx context.Context, rdb *redis.Client) {
	section("2/4 — HMGET: múltiplos campos em uma única chamada")

	explain("HMGET lê vários campos de uma vez numa única round-trip ao servidor.")
	explain("Muito mais eficiente do que fazer um HGET por campo.")
	fmt.Println()

	key := "tour:user:1"

	run(fmt.Sprintf("HMGET %s nome email cidade  ← 3 campos, 1 chamada", key))
	vals, _ := rdb.HMGet(ctx, key, "nome", "email", "cidade").Result()
	for i, v := range vals {
		if v == nil {
			result(fmt.Sprintf("%d) (nil)", i+1))
		} else {
			result(fmt.Sprintf("%d) %q", i+1, v))
		}
	}

	fmt.Println()

	// Mistura de campos existentes e inexistentes
	run(fmt.Sprintf("HMGET %s nome telefone plano  ← campo inexistente retorna nil", key))
	vals, _ = rdb.HMGet(ctx, key, "nome", "telefone", "plano").Result()
	for i, v := range vals {
		if v == nil {
			result(fmt.Sprintf("%d) (nil)", i+1))
		} else {
			result(fmt.Sprintf("%d) %q", i+1, v))
		}
	}

	explain("\n💡 A posição do nil corresponde ao campo que não existe — a ordem é sempre preservada.")
}

// ── Parte 3: HINCRBY ─────────────────────────────────────────────────────────

func part3Hincrby(ctx context.Context, rdb *redis.Client) {
	section("3/4 — HINCRBY: sistema de pontos")

	explain("HINCRBY incrementa um campo numérico dentro do hash atomicamente.")
	explain("Útil para contadores por objeto: pontos, views, likes, estoque...")
	fmt.Println()

	// Cria dois usuários para simular uma partida
	rdb.HSet(ctx, "tour:user:1", "pontos", 0)
	rdb.HSet(ctx, "tour:user:2", map[string]any{
		"nome":   "Bruno Santos",
		"email":  "bruno@example.com",
		"pontos": 0,
	})

	explain("Estado inicial:")
	for _, uid := range []string{"tour:user:1", "tour:user:2"} {
		nome, _ := rdb.HGet(ctx, uid, "nome").Result()
		pontos, _ := rdb.HGet(ctx, uid, "pontos").Result()
		explain(fmt.Sprintf("   %-15s pontos: %s", nome, pontos))
	}
	fmt.Println()

	type evento struct {
		user  string
		key   string
		delta int64
		desc  string
	}

	eventos := []evento{
		{"Ana Lima", "tour:user:1", 10, "acertou pergunta fácil"},
		{"Bruno Santos", "tour:user:2", 10, "acertou pergunta fácil"},
		{"Ana Lima", "tour:user:1", 25, "acertou pergunta difícil"},
		{"Bruno Santos", "tour:user:2", -5, "resposta errada"},
		{"Ana Lima", "tour:user:1", 15, "bônus de velocidade"},
		{"Bruno Santos", "tour:user:2", 30, "acertou pergunta difícil"},
	}

	for _, ev := range eventos {
		var userKey string
		if ev.user == "Ana Lima" {
			userKey = "tour:user:1"
		} else {
			userKey = "tour:user:2"
		}

		run(fmt.Sprintf("HINCRBY %s pontos %+d  ← %s: %s", userKey, ev.delta, ev.user, ev.desc))
		novosPontos, _ := rdb.HIncrBy(ctx, ev.key, "pontos", ev.delta).Result()
		result(fmt.Sprintf("(integer) %d", novosPontos))
		time.Sleep(120 * time.Millisecond)
	}

	fmt.Println()
	explain("Placar final:")
	for _, uid := range []string{"tour:user:1", "tour:user:2"} {
		nome, _ := rdb.HGet(ctx, uid, "nome").Result()
		pontos, _ := rdb.HGet(ctx, uid, "pontos").Result()
		explain(fmt.Sprintf("   %-15s pontos: %s", nome, pontos))
	}

	explain("\n💡 HINCRBYFLOAT faz o mesmo para valores decimais (ex: preços, saldos).")
}

// ── Parte 4: HDEL / HEXISTS ──────────────────────────────────────────────────

func part4HdelHexists(ctx context.Context, rdb *redis.Client) {
	section("4/4 — HDEL e HEXISTS: remover campos e verificar existência")

	explain("HDEL remove um ou mais campos do hash sem apagar o hash inteiro.")
	explain("HEXISTS verifica se um campo existe (1) ou não (0).")
	fmt.Println()

	key := "tour:user:1"

	// Estado atual
	run(fmt.Sprintf("HGETALL %s  ← estado atual", key))
	fields, _ := rdb.HGetAll(ctx, key).Result()
	i := 1
	for campo, valor := range fields {
		result(fmt.Sprintf("%d) %-10s  %s", i, campo, valor))
		i++
	}

	fmt.Println()

	// HEXISTS antes de remover
	run(fmt.Sprintf("HEXISTS %s cidade  ← o campo existe?", key))
	exists, _ := rdb.HExists(ctx, key, "cidade").Result()
	if exists {
		result("(integer) 1  ← existe ✅")
	} else {
		result("(integer) 0  ← não existe")
	}

	// HDEL de um campo
	fmt.Println()
	run(fmt.Sprintf("HDEL %s cidade  ← remove campo cidade", key))
	deleted, _ := rdb.HDel(ctx, key, "cidade").Result()
	result(fmt.Sprintf("(integer) %d  ← campos removidos", deleted))

	// HEXISTS depois
	run(fmt.Sprintf("HEXISTS %s cidade  ← agora?", key))
	exists, _ = rdb.HExists(ctx, key, "cidade").Result()
	if exists {
		result("(integer) 1  ← existe")
	} else {
		result("(integer) 0  ← não existe mais 🗑️")
	}

	// HDEL de múltiplos campos
	fmt.Println()
	run(fmt.Sprintf("HDEL %s ativo idade  ← remove múltiplos campos", key))
	deleted, _ = rdb.HDel(ctx, key, "ativo", "idade").Result()
	result(fmt.Sprintf("(integer) %d  ← campos removidos", deleted))

	// Estado final
	fmt.Println()
	run(fmt.Sprintf("HGETALL %s  ← estado final", key))
	fields, _ = rdb.HGetAll(ctx, key).Result()
	j := 1
	for campo, valor := range fields {
		result(fmt.Sprintf("%d) %-10s  %s", j, campo, valor))
		j++
	}

	explain("\n💡 Para apagar o hash inteiro, use DEL tour:user:1 — o mesmo comando de qualquer chave.")
}

