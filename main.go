package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrl00/redis-example/demos"
	"github.com/redis/go-redis/v9"
)

const redisAddr = "localhost:6379"

func main() {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	// ── Healthcheck ──────────────────────────────────────────
	start := time.Now()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Printf("\n❌  Não foi possível conectar ao Redis em %s\n", redisAddr)
		fmt.Println("   Verifique se o container está rodando: docker compose up -d")
		os.Exit(1)
	}
	latency := time.Since(start)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║         Redis Tour — Go Edition      ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Printf("✅  Conectado em %s  (latência: %s)\n\n", redisAddr, latency.Round(time.Microsecond))

	scanner := bufio.NewScanner(os.Stdin)

	for {
		printMenu()
		fmt.Print("› ")

		if !scanner.Scan() {
			break
		}

		choice := strings.TrimSpace(scanner.Text())
		fmt.Println()

		switch choice {
		case "1":
			demos.RunStrings(ctx, rdb)
		case "2":
			demos.RunLists(ctx, rdb)
		case "3":
			demos.RunHashes(ctx, rdb)
		case "4":
			demos.RunSets(ctx, rdb)
		case "5":
			fmt.Println("🔜  Em breve — Sorted Sets & Leaderboard")
		case "6":
			fmt.Println("🔜  Em breve — Cache com TTL")
		case "7":
			fmt.Println("🔜  Em breve — Rate Limiter")
		case "8":
			fmt.Println("🔜  Em breve — Session Store")
		case "0":
			fmt.Println("Até mais! 👋")
			return
		default:
			fmt.Println("Opção inválida. Escolha entre 0 e 8.")
		}

		// Após cada demo, mostra as chaves e oferece flush
		if choice >= "1" && choice <= "8" && choice != "2" {
			postDemo(ctx, rdb, scanner)
		}

		fmt.Println()
	}
}

func printMenu() {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│  Escolha uma demo                   │")
	fmt.Println("├─────────────────────────────────────┤")
	fmt.Println("│  [1] Strings & contadores            │")
	fmt.Println("│  [2] Lists & filas                  │")
	fmt.Println("│  [3] Hashes & objetos               │")
	fmt.Println("│  [4] Sets & conjuntos               │")
	fmt.Println("│  [5] Sorted Sets & leaderboard      │")
	fmt.Println("│  [6] Cache com TTL                  │")
	fmt.Println("│  [7] Rate limiter                   │")
	fmt.Println("│  [8] Session store                  │")
	fmt.Println("│  [0] Sair                           │")
	fmt.Println("└─────────────────────────────────────┘")
}

// postDemo exibe as chaves criadas e pergunta se quer limpar o Redis.
func postDemo(ctx context.Context, rdb *redis.Client, scanner *bufio.Scanner) {
	fmt.Println()

	keys, err := rdb.Keys(ctx, "*").Result()
	if err == nil && len(keys) > 0 {
		fmt.Printf("🔑  Chaves no Redis agora (%d):\n", len(keys))
		for _, k := range keys {
			t, _ := rdb.Type(ctx, k).Result()
			ttl, _ := rdb.TTL(ctx, k).Result()
			ttlStr := "sem expiração"
			if ttl > 0 {
				ttlStr = fmt.Sprintf("expira em %s", ttl.Round(time.Second))
			}
			fmt.Printf("    %-30s  tipo: %-10s  %s\n", k, t, ttlStr)
		}
	}

	fmt.Println()
	fmt.Print("🗑️   Limpar Redis antes de continuar? (s/N) ")
	if !scanner.Scan() {
		return
	}
	if strings.ToLower(strings.TrimSpace(scanner.Text())) == "s" {
		rdb.FlushAll(ctx)
		fmt.Println("   ✅  Redis limpo.")
	}
}
