# Redis Tour — Go Edition

Aplicação de linha de comando que demonstra os principais conceitos do Redis na prática, usando Go e a biblioteca [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis).

---

## Pré-requisitos

- [Go 1.26+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) e Docker Compose
- Make

---

## Rodando o projeto

```bash
make
```

Isso sobe o Redis via Docker Compose, aguarda o healthcheck passar e executa a aplicação.

---

## Serviços Docker

| Container       | Porta  | Descrição                  |
|-----------------|--------|----------------------------|
| `redis-tour`    | `6379` | Redis 7.2 (Alpine)         |
| `redis-tour-ui` | `5540` | RedisInsight — GUI oficial |

Acesse o RedisInsight em [http://localhost:5540](http://localhost:5540) para visualizar as chaves enquanto roda as demos.

Verificar se está rodando:

```bash
docker compose ps
docker compose exec redis redis-cli PING
# +PONG
```

---

## Comandos Make

```bash
make        # sobe o Redis e executa a aplicação
make down   # para os containers
make logs   # acompanha os logs do Redis em tempo real
make clean  # para os containers e apaga o volume de dados
```

---

Você verá o menu:

```
╔══════════════════════════════════════╗
║         Redis Tour — Go Edition      ║
╚══════════════════════════════════════╝
✅  Conectado em localhost:6379  (latência: 312µs)

┌─────────────────────────────────────┐
│  Escolha uma demo                   │
├─────────────────────────────────────┤
│  [1] Strings & contadores           │
│  [2] Lists & filas                  │
│  [3] Hashes & objetos               │
│  [4] Sets & conjuntos               │
│  [5] Sorted Sets & leaderboard      │
│  [6] Cache com TTL                  │
│  [7] Rate limiter                   │
│  [8] Session store                  │
│  [0] Sair                           │
└─────────────────────────────────────┘
```

Ao final de cada demo o programa lista todas as chaves criadas no Redis e pergunta se você quer limpar o banco antes de continuar.

---

## Estrutura do projeto

```
redis-tour/
├── docker-compose.yml
├── Makefile
├── go.mod
├── main.go           # conexão, menu e pós-demo (KEYS + FLUSHALL)
└── demos/
    ├── strings.go
    ├── lists.go
    ├── hashes.go
    ├── sets.go
    ├── sorted_sets.go
    └── usecases.go   # cache com TTL, rate limiter e session store
```

---

## O que cada demo cobre

### [1] Strings & contadores
| Comando | O que demonstra |
|---|---|
| `SET` / `GET` | Armazenar e recuperar valores, sobrescrita, chave inexistente |
| `INCR` / `INCRBY` / `DECRBY` | Contador atômico simulando visitas |
| `SET ... EX` + `TTL` | Expiração automática com monitoramento do TTL em tempo real |
| `SET ... NX EX` | Lock distribuído simples com dois workers competindo |

### [2] Lists & filas
| Comando | O que demonstra |
|---|---|
| `RPUSH` / `LRANGE` / `LLEN` | Montar e inspecionar uma fila de e-mails |
| `LPOP` / `RPOP` | Consumir elementos em ordem FIFO e LIFO |
| `LPUSH` + `LPOP` | List como pilha — histórico de navegação com botão "voltar" |
| `BLPOP` | Worker bloqueante aguardando jobs chegarem na fila |

### [3] Hashes & objetos
| Comando | O que demonstra |
|---|---|
| `HSET` / `HGET` / `HGETALL` | Criar e ler perfil de usuário com múltiplos campos |
| `HKEYS` / `HVALS` / `HLEN` | Inspecionar estrutura do hash |
| `HMGET` | Ler múltiplos campos em uma única round-trip |
| `HINCRBY` | Sistema de pontos — placar ao vivo entre dois jogadores |
| `HDEL` / `HEXISTS` | Remover campos individualmente e verificar existência |

### [4] Sets & conjuntos
| Comando | O que demonstra |
|---|---|
| `SADD` / `SMEMBERS` / `SCARD` | Criar conjuntos de tags, ignorar duplicatas |
| `SISMEMBER` / `SMISMEMBER` | Lista negra de IPs — verificar pertencimento em O(1) |
| `SINTER` / `SUNION` / `SDIFF` | Tópicos em comum entre usuários, `SINTERSTORE` |
| `SRANDMEMBER` / `SPOP` | Amostragem e sorteio de vencedores sem repetição |

### [5] Sorted Sets & leaderboard
| Comando | O que demonstra |
|---|---|
| `ZADD` / `ZREVRANGE` / `ZCARD` | Leaderboard com 7 jogadores e desempate lexicográfico |
| `ZSCORE` / `ZREVRANK` | Consultar score e posição de um jogador específico |
| `ZINCRBY` | Partida ao vivo — kills, penalidades e MVP reordenam o ranking |
| `ZRANGEBYSCORE` / `ZCOUNT` / `ZREMRANGEBYSCORE` | Filtrar por faixa, contar e rebaixar jogadores |

### [6] Cache com TTL
| Comando | O que demonstra |
|---|---|
| `GET` → miss → `SET ... EX` | Padrão cache-aside: busca no "banco" (250ms), armazena no Redis |
| `GET` → hit | Segunda e terceira requisições retornam em microssegundos |
| `TTL` com monitoramento | TTL curto de 3s observado em tempo real até expirar |
| Repovoamento automático | Miss após expiração busca dados frescos e repovoaa cache |

### [7] Rate Limiter
| Comando | O que demonstra |
|---|---|
| `INCR` + `EXPIRE` | Fixed window — 8 requisições com limite de 5, bloqueadas mostram TTL restante |
| `ZADD` + `ZREMRANGEBYSCORE` + `ZCARD` | Sliding window — janela móvel de 10s com timestamps, sem problema de rajada |

### [8] Session Store
| Comando | O que demonstra |
|---|---|
| `HSET` + `EXPIRE` | Login — cria sessão com 6 campos e TTL de inatividade |
| `EXISTS` + `HGETALL` + `HGET` | Requisições autenticadas — valida token, lê dados, verifica role |
| `EXPIRE` (renovação) | Sliding expiration — TTL resetado a cada requisição autenticada |
| `DEL` | Logout — remove sessão imediatamente; token posterior retorna empty |

---

## Comandos úteis

```bash
# Abrir o Redis CLI no container
docker compose exec redis redis-cli

# Monitorar todos os comandos em tempo real (abra num segundo terminal)
docker compose exec redis redis-cli MONITOR

# Listar todas as chaves
docker compose exec redis redis-cli KEYS '*'

# Limpar o banco
docker compose exec redis redis-cli FLUSHALL

# Parar os containers
docker compose down

# Parar e apagar o volume (reseta os dados)
docker compose down -v
```

---

## Referências

- [Documentação oficial do Redis](https://redis.io/docs/)
- [go-redis — documentação](https://redis.uptrace.dev/)
- [Redis University (cursos gratuitos)](https://university.redis.io/)
