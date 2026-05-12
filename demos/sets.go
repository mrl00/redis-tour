package demos

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunSets executa a demo de Sets em 4 partes:
//  1. SADD / SMEMBERS / SCARD  — criar conjuntos e inspecionar
//  2. SISMEMBER / SMISMEMBER   — verificar pertencimento
//  3. SINTER / SUNION / SDIFF  — operações de conjunto
//  4. SRANDMEMBER / SPOP       — elementos aleatórios (sorteio)
func RunSets(ctx context.Context, rdb *redis.Client) {
	header("Sets & Conjuntos")

	part1BuildSets(ctx, rdb)
	pause()

	part2Membership(ctx, rdb)
	pause()

	part3SetOps(ctx, rdb)
	pause()

	part4Random(ctx, rdb)

	fmt.Println("\n✅  Demo de Sets concluída.")
}

// ── Parte 1: SADD / SMEMBERS / SCARD ─────────────────────────────────────────

func part1BuildSets(ctx context.Context, rdb *redis.Client) {
	section("1/4 — SADD, SMEMBERS e SCARD")

	explain("Set é uma coleção não ordenada de strings únicas.")
	explain("Duplicatas são silenciosamente ignoradas — o Redis não retorna erro.")
	explain("Operações de leitura e escrita em O(1) para elementos individuais.")
	fmt.Println()

	keyA := "tour:set:tags:produto:1"
	keyB := "tour:set:tags:produto:2"

	rdb.Del(ctx, keyA, keyB)

	// Produto 1
	tagsP1 := []string{"eletrônico", "smartphone", "android", "5g", "câmera"}
	run(fmt.Sprintf("SADD %s %v", keyA, tagsP1))
	n, _ := rdb.SAdd(ctx, keyA, toAny(tagsP1)...).Result()
	result(fmt.Sprintf("(integer) %d  ← membros adicionados", n))

	// Tenta adicionar duplicata
	fmt.Println()
	run(fmt.Sprintf("SADD %s \"smartphone\"  ← tentando duplicata", keyA))
	n, _ = rdb.SAdd(ctx, keyA, "smartphone").Result()
	result(fmt.Sprintf("(integer) %d  ← 0 = já existia, não foi adicionado", n))

	fmt.Println()

	// Produto 2
	tagsP2 := []string{"eletrônico", "tablet", "android", "wifi", "câmera"}
	run(fmt.Sprintf("SADD %s %v", keyB, tagsP2))
	n, _ = rdb.SAdd(ctx, keyB, toAny(tagsP2)...).Result()
	result(fmt.Sprintf("(integer) %d  ← membros adicionados", n))

	fmt.Println()

	// SMEMBERS
	run(fmt.Sprintf("SMEMBERS %s", keyA))
	members, _ := rdb.SMembers(ctx, keyA).Result()
	for i, m := range members {
		result(fmt.Sprintf("%d) %q", i+1, m))
	}

	fmt.Println()

	// SCARD — cardinalidade (tamanho)
	run(fmt.Sprintf("SCARD %s", keyA))
	card, _ := rdb.SCard(ctx, keyA).Result()
	result(fmt.Sprintf("(integer) %d", card))

	explain("\n💡 A ordem dos SMEMBERS não é garantida — Sets não são ordenados.")
	explain("   Se precisar de ordem, use Sorted Set (demo 5).")
}

// ── Parte 2: SISMEMBER / SMISMEMBER ──────────────────────────────────────────

func part2Membership(ctx context.Context, rdb *redis.Client) {
	section("2/4 — SISMEMBER e SMISMEMBER: verificar pertencimento em O(1)")

	explain("Checar se um elemento pertence a um Set é O(1) — independente do tamanho.")
	explain("É um dos casos de uso mais comuns: listas negras, permissões, flags...")
	fmt.Println()

	keyBan := "tour:set:ips:bloqueados"
	rdb.Del(ctx, keyBan)

	ips := []string{"192.168.1.50", "10.0.0.99", "172.16.0.200", "203.0.113.42"}
	run(fmt.Sprintf("SADD %s  (lista negra de IPs)", keyBan))
	rdb.SAdd(ctx, keyBan, toAny(ips)...)
	for _, ip := range ips {
		result(fmt.Sprintf("   + %s", ip))
	}

	fmt.Println()

	// SISMEMBER
	checks := []struct {
		ip      string
		blocked bool
	}{
		{"192.168.1.50", true},
		{"8.8.8.8", false},
		{"203.0.113.42", true},
		{"192.168.1.1", false},
	}

	for _, c := range checks {
		run(fmt.Sprintf("SISMEMBER %s \"%s\"", keyBan, c.ip))
		is, _ := rdb.SIsMember(ctx, keyBan, c.ip).Result()
		if is {
			result(fmt.Sprintf("(integer) 1  ← bloqueado 🚫"))
		} else {
			result(fmt.Sprintf("(integer) 0  ← permitido ✅"))
		}
		time.Sleep(80 * time.Millisecond)
	}

	fmt.Println()

	// SMISMEMBER — checar múltiplos de uma vez (Redis 6.2+)
	run(fmt.Sprintf("SMISMEMBER %s \"8.8.8.8\" \"10.0.0.99\" \"1.1.1.1\"  ← 3 de uma vez", keyBan))
	results, _ := rdb.SMIsMember(ctx, keyBan, "8.8.8.8", "10.0.0.99", "1.1.1.1").Result()
	labels := []string{"8.8.8.8", "10.0.0.99", "1.1.1.1"}
	for i, r := range results {
		blocked := 0
		if r {
			blocked = 1
		}
		result(fmt.Sprintf("%d) (integer) %d  ← %s", i+1, blocked, labels[i]))
	}

	explain("\n💡 SMISMEMBER é mais eficiente do que N chamadas a SISMEMBER — uma só round-trip.")
}

// ── Parte 3: SINTER / SUNION / SDIFF ─────────────────────────────────────────

func part3SetOps(ctx context.Context, rdb *redis.Client) {
	section("3/4 — SINTER, SUNION e SDIFF: operações de conjunto")

	explain("Redis executa álgebra de conjuntos direto no servidor — sem trazer os dados.")
	explain("Muito usado em: amigos em comum, recomendações, filtros de permissão...")
	fmt.Println()

	u1 := "tour:set:seguindo:ana"
	u2 := "tour:set:seguindo:bruno"
	u3 := "tour:set:seguindo:carla"

	rdb.Del(ctx, u1, u2, u3)

	rdb.SAdd(ctx, u1, "redis", "golang", "docker", "kubernetes", "postgres")
	rdb.SAdd(ctx, u2, "redis", "golang", "python", "docker", "kafka")
	rdb.SAdd(ctx, u3, "redis", "rust", "kafka", "grpc", "postgres")

	explain("Tópicos seguidos por cada usuário:")
	explain("   Ana:   redis, golang, docker, kubernetes, postgres")
	explain("   Bruno: redis, golang, python, docker, kafka")
	explain("   Carla: redis, rust, kafka, grpc, postgres")
	fmt.Println()

	// SINTER — interseção (em comum entre todos)
	run(fmt.Sprintf("SINTER %s %s %s  ← em comum entre os 3", u1, u2, u3))
	inter, _ := rdb.SInter(ctx, u1, u2, u3).Result()
	for i, m := range inter {
		result(fmt.Sprintf("%d) %q", i+1, m))
	}
	explain("   → tópicos que Ana, Bruno E Carla seguem")

	fmt.Println()

	// SINTER só entre dois
	run(fmt.Sprintf("SINTER %s %s  ← em comum entre Ana e Bruno", u1, u2))
	inter2, _ := rdb.SInter(ctx, u1, u2).Result()
	for i, m := range inter2 {
		result(fmt.Sprintf("%d) %q", i+1, m))
	}

	fmt.Println()

	// SUNION — união (todos os tópicos sem repetição)
	run(fmt.Sprintf("SUNION %s %s %s  ← todos os tópicos (sem duplicatas)", u1, u2, u3))
	union, _ := rdb.SUnion(ctx, u1, u2, u3).Result()
	for i, m := range union {
		result(fmt.Sprintf("%d) %q", i+1, m))
	}

	fmt.Println()

	// SDIFF — diferença (o que Ana segue e Bruno/Carla não seguem)
	run(fmt.Sprintf("SDIFF %s %s %s  ← o que só Ana segue", u1, u2, u3))
	diff, _ := rdb.SDiff(ctx, u1, u2, u3).Result()
	for i, m := range diff {
		result(fmt.Sprintf("%d) %q", i+1, m))
	}
	explain("   → tópicos exclusivos de Ana (não seguidos por Bruno nem Carla)")

	fmt.Println()

	// SINTERSTORE — salva o resultado em uma nova chave
	destKey := "tour:set:comum:ana-bruno"
	run(fmt.Sprintf("SINTERSTORE %s %s %s  ← salva interseção numa nova chave", destKey, u1, u2))
	stored, _ := rdb.SInterStore(ctx, destKey, u1, u2).Result()
	result(fmt.Sprintf("(integer) %d  ← membros salvos em %s", stored, destKey))

	explain("\n💡 SINTERSTORE / SUNIONSTORE / SDIFFSTORE salvam o resultado para reutilizar depois.")
}

// ── Parte 4: SRANDMEMBER / SPOP ──────────────────────────────────────────────

func part4Random(ctx context.Context, rdb *redis.Client) {
	section("4/4 — SRANDMEMBER e SPOP: sorteio e amostragem")

	explain("SRANDMEMBER retorna elemento(s) aleatório(s) sem remover do Set.")
	explain("SPOP retorna E remove — perfeito para sorteios sem repetição.")
	fmt.Println()

	key := "tour:set:participantes"
	rdb.Del(ctx, key)

	participantes := []string{
		"Alice", "Bruno", "Carla", "Diego",
		"Eva", "Felipe", "Gabriela", "Henrique",
	}

	run(fmt.Sprintf("SADD %s  (%d participantes)", key, len(participantes)))
	rdb.SAdd(ctx, key, toAny(participantes)...)
	for _, p := range participantes {
		result(fmt.Sprintf("   + %s", p))
	}

	fmt.Println()

	// SRANDMEMBER — sorteia sem remover
	run(fmt.Sprintf("SRANDMEMBER %s 3  ← 3 nomes aleatórios (não remove)", key))
	sample, _ := rdb.SRandMemberN(ctx, key, 3).Result()
	for i, m := range sample {
		result(fmt.Sprintf("%d) %q", i+1, m))
	}

	run(fmt.Sprintf("SCARD %s  ← tamanho não mudou", key))
	card, _ := rdb.SCard(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d  ← ainda %d participantes", card, card))

	fmt.Println()

	// SPOP — sorteio sem repetição (remove ao sortear)
	explain("Sorteando 3 vencedores sem repetição com SPOP:")
	fmt.Println()

	lugares := []string{"🥇 1º lugar", "🥈 2º lugar", "🥉 3º lugar"}
	for _, lugar := range lugares {
		run(fmt.Sprintf("SPOP %s", key))
		vencedor, _ := rdb.SPop(ctx, key).Result()
		result(fmt.Sprintf("%q  ← %s: %s", vencedor, lugar, vencedor))
		remaining, _ := rdb.SCard(ctx, key).Result()
		explain(fmt.Sprintf("   restam %d participantes no sorteio", remaining))
		fmt.Println()
		time.Sleep(300 * time.Millisecond)
	}

	explain("💡 SPOP garante que o mesmo elemento não seja sorteado duas vezes.")
	explain("   SRANDMEMBER com count negativo pode retornar duplicatas — útil para amostras.")
}

// ── Helper ───────────────────────────────────────────────────────────────────

func toAny(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
