package demos

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunSortedSets executa a demo de Sorted Sets em 4 partes:
//  1. ZADD / ZRANGE / ZCARD     — montar e inspecionar um leaderboard
//  2. ZRANK / ZREVRANK / ZSCORE — posição e pontuação de um jogador
//  3. ZINCRBY                   — simular partidas atualizando scores ao vivo
//  4. ZRANGEBYSCORE / ZREMRANGEBYSCORE — filtrar e limpar por faixa de score
func RunSortedSets(ctx context.Context, rdb *redis.Client) {
	header("Sorted Sets & Leaderboard")

	part1BuildLeaderboard(ctx, rdb)
	pause()

	part2RankScore(ctx, rdb)
	pause()

	part3LiveMatch(ctx, rdb)
	pause()

	part4RangeByScore(ctx, rdb)

	fmt.Println("\n✅  Demo de Sorted Sets concluída.")
}

// ── Parte 1: ZADD / ZRANGE / ZCARD ───────────────────────────────────────────

func part1BuildLeaderboard(ctx context.Context, rdb *redis.Client) {
	section("1/4 — ZADD, ZRANGE e ZCARD: montando o leaderboard")

	explain("Sorted Set é como um Set, mas cada membro tem um score float64.")
	explain("O Redis mantém os membros sempre ordenados pelo score automaticamente.")
	explain("Inserção e busca em O(log N) — eficiente mesmo com milhões de jogadores.")
	fmt.Println()

	key := "tour:zset:leaderboard"
	rdb.Del(ctx, key)

	type jogador struct {
		nome  string
		score float64
	}

	jogadores := []jogador{
		{"Alice", 9800},
		{"Bruno", 7500},
		{"Carla", 12300},
		{"Diego", 8900},
		{"Eva", 11100},
		{"Felipe", 7500}, // mesmo score que Bruno — desempate lexicográfico
		{"Gabriela", 15000},
	}

	explain("Adicionando jogadores com seus scores:")
	fmt.Println()

	for _, j := range jogadores {
		run(fmt.Sprintf("ZADD %s %.0f \"%s\"", key, j.score, j.nome))
		rdb.ZAdd(ctx, key, redis.Z{Score: j.score, Member: j.nome})
		result(fmt.Sprintf("(integer) 1"))
		time.Sleep(60 * time.Millisecond)
	}

	fmt.Println()

	// ZCARD
	run(fmt.Sprintf("ZCARD %s  ← total de membros", key))
	card, _ := rdb.ZCard(ctx, key).Result()
	result(fmt.Sprintf("(integer) %d", card))

	fmt.Println()

	// ZRANGE crescente (menor → maior score)
	run(fmt.Sprintf("ZRANGE %s 0 -1 WITHSCORES  ← ordem crescente (pior → melhor)", key))
	members, _ := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	for i, m := range members {
		result(fmt.Sprintf("%d) %-12s  %.0f pts", i+1, m.Member, m.Score))
	}

	fmt.Println()

	// ZREVRANGE decrescente (maior → menor score) — ranking real
	run(fmt.Sprintf("ZREVRANGE %s 0 -1 WITHSCORES  ← ordem decrescente (melhor → pior)", key))
	rev, _ := rdb.ZRevRangeWithScores(ctx, key, 0, -1).Result()
	for i, m := range rev {
		medal := "   "
		switch i {
		case 0:
			medal = "🥇 "
		case 1:
			medal = "🥈 "
		case 2:
			medal = "🥉 "
		}
		result(fmt.Sprintf("%s#%d  %-12s  %.0f pts", medal, i+1, m.Member, m.Score))
	}

	explain("\n💡 Bruno e Felipe têm o mesmo score — o Redis desempata em ordem lexicográfica.")
}

// ── Parte 2: ZRANK / ZREVRANK / ZSCORE ───────────────────────────────────────

func part2RankScore(ctx context.Context, rdb *redis.Client) {
	section("2/4 — ZRANK, ZREVRANK e ZSCORE: consultando um jogador")

	explain("ZRANK retorna a posição 0-based em ordem crescente (0 = menor score).")
	explain("ZREVRANK retorna a posição em ordem decrescente (0 = maior score = 1º lugar).")
	explain("ZSCORE retorna o score atual de um membro.")
	fmt.Println()

	key := "tour:zset:leaderboard"

	jogadores := []string{"Gabriela", "Eva", "Alice", "Bruno"}

	for _, nome := range jogadores {
		run(fmt.Sprintf("ZSCORE %s \"%s\"", key, nome))
		score, _ := rdb.ZScore(ctx, key, nome).Result()
		result(fmt.Sprintf("\"%.0f\"", score))

		run(fmt.Sprintf("ZREVRANK %s \"%s\"  ← posição no ranking (0-based)", key, nome))
		rank, _ := rdb.ZRevRank(ctx, key, nome).Result()
		result(fmt.Sprintf("(integer) %d  ← %dº lugar", rank, rank+1))

		fmt.Println()
		time.Sleep(100 * time.Millisecond)
	}

	// Membro inexistente
	run(fmt.Sprintf("ZSCORE %s \"Zé Ninguém\"  ← membro inexistente", key))
	_, err := rdb.ZScore(ctx, key, "Zé Ninguém").Result()
	if err == redis.Nil {
		result("(nil)")
	}

	run(fmt.Sprintf("ZREVRANK %s \"Zé Ninguém\"", key))
	_, err = rdb.ZRevRank(ctx, key, "Zé Ninguém").Result()
	if err == redis.Nil {
		result("(nil)")
	}

	explain("\n💡 ZRANK/ZREVRANK retornam nil se o membro não existir — sem erro, sem panic.")
}

// ── Parte 3: ZINCRBY — partida ao vivo ───────────────────────────────────────

func part3LiveMatch(ctx context.Context, rdb *redis.Client) {
	section("3/4 — ZINCRBY: simulando uma partida ao vivo")

	explain("ZINCRBY incrementa (ou decrementa) o score de um membro atomicamente.")
	explain("O Sorted Set se reordena automaticamente após cada atualização.")
	fmt.Println()

	key := "tour:zset:leaderboard"

	type evento struct {
		jogador string
		delta   float64
		desc    string
	}

	eventos := []evento{
		{"Alice", +500, "kill streak"},
		{"Bruno", +1200, "capturou a base"},
		{"Carla", -400, "penalidade"},
		{"Eva", +800, "headshot x5"},
		{"Bruno", +600, "assistência"},
		{"Alice", +1100, "round MVP"},
		{"Gabriela", -200, "fogo amigo"},
		{"Diego", +900, "defesa perfeita"},
		{"Eva", +700, "clutch 1v3"},
	}

	for _, ev := range eventos {
		sinal := "+"
		if ev.delta < 0 {
			sinal = ""
		}
		run(fmt.Sprintf("ZINCRBY %s %s%.0f \"%s\"  ← %s", key, sinal, ev.delta, ev.jogador, ev.desc))
		novoScore, _ := rdb.ZIncrBy(ctx, key, ev.delta, ev.jogador).Result()

		rank, _ := rdb.ZRevRank(ctx, key, ev.jogador).Result()
		result(fmt.Sprintf("\"%.0f\"  → %s agora em %.0f pts  (#%d)", novoScore, ev.jogador, novoScore, rank+1))
		time.Sleep(200 * time.Millisecond)
	}

	// Ranking final
	fmt.Println()
	explain("Ranking final após a partida:")
	fmt.Println()

	run(fmt.Sprintf("ZREVRANGE %s 0 -1 WITHSCORES", key))
	rev, _ := rdb.ZRevRangeWithScores(ctx, key, 0, -1).Result()
	for i, m := range rev {
		medal := "   "
		switch i {
		case 0:
			medal = "🥇 "
		case 1:
			medal = "🥈 "
		case 2:
			medal = "🥉 "
		}
		result(fmt.Sprintf("%s#%d  %-12s  %.0f pts", medal, i+1, m.Member, m.Score))
	}

	explain("\n💡 ZINCRBY com valor negativo funciona como decremento — útil para penalidades.")
}

// ── Parte 4: ZRANGEBYSCORE / ZREMRANGEBYSCORE ─────────────────────────────────

func part4RangeByScore(ctx context.Context, rdb *redis.Client) {
	section("4/4 — ZRANGEBYSCORE e ZREMRANGEBYSCORE: filtrar por faixa de score")

	explain("ZRANGEBYSCORE retorna membros dentro de uma faixa de scores.")
	explain("Útil para: paginação de ranking, listar jogadores numa liga, expirar registros antigos.")
	fmt.Println()

	key := "tour:zset:leaderboard"

	// Snapshot do ranking atual
	run(fmt.Sprintf("ZRANGE %s 0 -1 WITHSCORES  ← scores atuais", key))
	all, _ := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	for _, m := range all {
		result(fmt.Sprintf("   %-12s  %.0f pts", m.Member, m.Score))
	}

	fmt.Println()

	// Faixa intermediária
	run(fmt.Sprintf("ZRANGEBYSCORE %s 8000 12000 WITHSCORES  ← entre 8k e 12k pts", key))
	faixa, _ := rdb.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min: "8000",
		Max: "12000",
	}).Result()
	if len(faixa) == 0 {
		result("(empty array)")
	}
	for i, m := range faixa {
		result(fmt.Sprintf("%d) %-12s  %.0f pts", i+1, m.Member, m.Score))
	}

	fmt.Println()

	// Top 3 com ZREVRANGEBYSCORE + LIMIT
	run(fmt.Sprintf("ZREVRANGEBYSCORE %s +inf -inf WITHSCORES LIMIT 0 3  ← top 3", key))
	top3, _ := rdb.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  3,
	}).Result()
	for i, m := range top3 {
		result(fmt.Sprintf("%d) %-12s  %.0f pts", i+1, m.Member, m.Score))
	}

	fmt.Println()

	// ZCOUNT — quantos membros numa faixa, sem trazer os dados
	run(fmt.Sprintf("ZCOUNT %s 8000 +inf  ← quantos jogadores têm 8k+ pts", key))
	count, _ := rdb.ZCount(ctx, key, "8000", "+inf").Result()
	result(fmt.Sprintf("(integer) %d", count))

	fmt.Println()

	// ZREMRANGEBYSCORE — remove os jogadores com score mais baixo (divisão de liga)
	run(fmt.Sprintf("ZREMRANGEBYSCORE %s -inf 7999  ← rebaixa jogadores abaixo de 8k", key))
	removed, _ := rdb.ZRemRangeByScore(ctx, key, "-inf", "7999").Result()
	result(fmt.Sprintf("(integer) %d  ← membros removidos", removed))

	fmt.Println()
	run(fmt.Sprintf("ZRANGE %s 0 -1 WITHSCORES  ← ranking final (só Liga A)", key))
	final, _ := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	for _, m := range final {
		result(fmt.Sprintf("   %-12s  %.0f pts", m.Member, m.Score))
	}

	explain("\n💡 Use \"-inf\" e \"+inf\" como limites abertos — o Redis aceita esses literais diretamente.")
	explain("   ZRANGEBYLEX faz o mesmo mas comparando strings lexicograficamente (scores iguais).")
}
