.PHONY: run down logs clean

run:
	docker compose up -d
	@echo "Aguardando Redis ficar healthy..."
	@until docker inspect --format='{{.State.Health.Status}}' redis-tour 2>/dev/null | grep -q "healthy"; do \
		sleep 1; \
	done
	@echo "Redis pronto. Iniciando aplicação...\n"
	go run .

down:
	docker compose down

logs:
	docker compose logs -f redis

clean:
	docker compose down -v
