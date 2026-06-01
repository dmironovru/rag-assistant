.PHONY: ingest dev clean

ingest:
	@echo "🚀 Запуск индексации..."
	@go run cmd/ingest/main.go --config config/sources.yaml

dev:
	@echo "🔧 Dev-режим: обрабатываем только $(source)..."
	@go run cmd/ingest/main.go --config config/sources.yaml --filter $(source)

clean:
	@echo "🧹 Очистка..."
	@sudo -u postgres psql -d todo_db -c "TRUNCATE document_chunks;"

db-check:
	@go run cmd/ingest/main.go --config config/sources.yaml --check-db
