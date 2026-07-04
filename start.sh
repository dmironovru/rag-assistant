#!/bin/bash
set -e

# === ЦВЕТА ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${BLUE}[RAG]${NC} $1"; }
info() { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[✗]${NC} $1"; }

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_DIR"
mkdir -p logs

# ============================================================
# 1. ПРОВЕРКА DOCKER
# ============================================================
log "Проверка Docker..."
if ! command -v docker >/dev/null 2>&1; then
    err "Docker не установлен!"
    echo "Установи: curl -fsSL https://get.docker.com | sh"
    exit 1
fi
if ! docker info >/dev/null 2>&1; then
    err "Docker не запущен или нет прав"
    echo "Запусти: sudo systemctl start docker && sudo usermod -aG docker \$USER && newgrp docker"
    exit 1
fi
info "Docker OK"

# ============================================================
# 2. ПРОВЕРКА POSTGRESQL
# ============================================================
log "Проверка PostgreSQL..."

# Проверяем, установлен ли PostgreSQL
if ! command -v psql >/dev/null 2>&1; then
    err "PostgreSQL не установлен!"
    echo "Установи: sudo apt install postgresql postgresql-contrib"
    exit 1
fi

# Проверяем, запущен ли PostgreSQL
if ! pg_isready -q 2>/dev/null; then
    warn "PostgreSQL не запущен. Запускаю..."
    sudo systemctl start postgresql 2>/dev/null || sudo service postgresql start 2>/dev/null || true
    sleep 3
    if ! pg_isready -q 2>/dev/null; then
        err "Не удалось запустить PostgreSQL!"
        echo "Запусти вручную: sudo systemctl start postgresql"
        exit 1
    fi
fi
info "PostgreSQL запущен ✅"

# Проверяем базу данных
if ! sudo -u postgres psql -lqt 2>/dev/null | cut -d \| -f 1 | grep -qw rag_docs; then
    warn "База rag_docs не найдена. Создаю..."
    sudo -u postgres psql -c "CREATE DATABASE rag_docs;" 2>/dev/null || true
    sudo -u postgres psql -d rag_docs -c "CREATE EXTENSION IF NOT EXISTS vector;" 2>/dev/null || true
    info "База создана ✅"
else
    info "База данных найдена ✅"
fi

# === СОЗДАНИЕ ПОЛЬЗОВАТЕЛЯ И ПРАВ ===
log "Настройка прав доступа..."

# Создаём пользователя rag, если его нет
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='rag'" | grep -q 1; then
    sudo -u postgres psql -c "CREATE USER rag WITH PASSWORD 'ragpass';" 2>/dev/null || true
    info "Пользователь rag создан ✅"
fi

# Создаём таблицу, если её нет
sudo -u postgres psql -d rag_docs -c "
CREATE TABLE IF NOT EXISTS document_chunks (
    id SERIAL PRIMARY KEY,
    source VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768),
    created_at TIMESTAMP DEFAULT NOW()
);" 2>/dev/null || true

# === ГЛАВНОЕ: ДАЁМ ПРАВА НА ТАБЛИЦУ ===
sudo -u postgres psql -d rag_docs -c "ALTER TABLE document_chunks OWNER TO rag;" 2>/dev/null || true
sudo -u postgres psql -d rag_docs -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO rag;" 2>/dev/null || true
sudo -u postgres psql -d rag_docs -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO rag;" 2>/dev/null || true
sudo -u postgres psql -d rag_docs -c "GRANT USAGE ON SCHEMA public TO rag;" 2>/dev/null || true

# Создаём индекс
sudo -u postgres psql -d rag_docs -c "
CREATE INDEX IF NOT EXISTS idx_embedding ON document_chunks USING hnsw (embedding vector_cosine_ops);
" 2>/dev/null || true

info "Права доступа настроены ✅"

# ============================================================
# 3. ПРОВЕРКА OLLAMA
# ============================================================
log "Проверка Ollama..."
if ! command -v ollama >/dev/null 2>&1; then
    err "Ollama не установлен!"
    echo "Установи: curl -fsSL https://ollama.com/install.sh | sh"
    exit 1
fi

if ! pgrep -f "ollama serve" >/dev/null; then
    warn "Ollama не запущен. Запускаю..."
    ollama serve > /dev/null 2>&1 &
    sleep 5
    info "Ollama запущен ✅"
else
    info "Ollama уже работает ✅"
fi

# Проверяем модели
for model in "nomic-embed-text" "mistral:7b"; do
    if ! ollama list 2>/dev/null | grep -q "$model"; then
        log "Скачиваю модель $model (это может занять время)..."
        ollama pull "$model"
        info "Модель $model скачана ✅"
    else
        info "Модель $model уже есть ✅"
    fi
done

# ============================================================
# 4. ПРОВЕРКА GO
# ============================================================
if ! command -v go >/dev/null 2>&1; then
    err "Go не установлен!"
    echo "Установи: sudo apt install golang-go"
    exit 1
fi
info "Go OK ✅"

# ============================================================
# 5. ПРОВЕРКА NODE.JS
# ============================================================
if ! command -v node >/dev/null 2>&1; then
    err "Node.js не установлен!"
    echo "Установи: curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt install -y nodejs"
    exit 1
fi
info "Node.js OK ✅"

# ============================================================
# 6. ПРОВЕРКА ПОРТОВ
# ============================================================
log "Проверка портов..."
for port in 8080 3000 5432 11434; do
    if command -v lsof >/dev/null 2>&1; then
        if lsof -i :$port >/dev/null 2>&1; then
            pid=$(lsof -t -i :$port)
            warn "Порт $port занят (PID: $pid). Освобождаю..."
            kill -9 $pid 2>/dev/null || true
            sleep 1
        fi
    fi
done
info "Порты готовы ✅"

# ============================================================
# 7. ЗАПУСК БЭКЕНДА
# ============================================================
log "Запуск Go-бэкенда..."
cd "$PROJECT_DIR/backend"

# Создаём конфиг если нет
if [ ! -f "config/sources.yaml" ]; then
    mkdir -p config
    cat > config/sources.yaml << 'YAML'
db_connection: "postgres://rag:ragpass@localhost:5432/rag_docs"
ollama_url: "http://localhost:11434"
embed_model: "nomic-embed-text"
generate_model: "mistral:7b"
YAML
fi

export DB_CONN="postgres://rag:ragpass@localhost:5432/rag_docs"
export OLLAMA_URL="http://localhost:11434"

go run cmd/server/main.go > "$PROJECT_DIR/logs/backend.log" 2>&1 &
BACKEND_PID=$!
sleep 5

if grep -q "API Server запущен" "$PROJECT_DIR/logs/backend.log" 2>/dev/null; then
    info "Бэкенд запущен ✅ (PID: $BACKEND_PID)"
else
    err "Бэкенд не запустился!"
    tail -10 "$PROJECT_DIR/logs/backend.log"
    exit 1
fi

# ============================================================
# 8. ЗАПУСК ФРОНТЕНДА
# ============================================================
log "Запуск Next.js-фронтенда..."
cd "$PROJECT_DIR/frontend"

if [ ! -d "node_modules" ]; then
    log "Установка npm-зависимостей..."
    npm install --silent
fi

export RAG_API_URL="http://localhost:8080/api/search"
export OLLAMA_URL="http://localhost:11434/api/generate"

npm run dev > "$PROJECT_DIR/logs/frontend.log" 2>&1 &
FRONTEND_PID=$!
sleep 8

if curl -s http://localhost:3000 >/dev/null 2>&1; then
    info "Фронтенд запущен ✅ (PID: $FRONTEND_PID)"
else
    err "Фронтенд не запустился!"
    tail -10 "$PROJECT_DIR/logs/frontend.log"
    exit 1
fi

# ============================================================
# 9. ФИНАЛЬНОЕ СООБЩЕНИЕ
# ============================================================
echo ""
echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  🚀 RAG Assistant запущен!                               ║${NC}"
echo -e "${GREEN}╠═══════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  🌐 Frontend:  http://localhost:3000                     ║${NC}"
echo -e "${GREEN}║  🔌 Backend:   http://localhost:8080                     ║${NC}"
echo -e "${GREEN}║  🗄️  PostgreSQL: localhost:5432                          ║${NC}"
echo -e "${GREEN}║  🧠 Ollama:    http://localhost:11434                    ║${NC}"
echo -e "${GREEN}╠═══════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  📋 Логи:     tail -f logs/backend.log                   ║${NC}"
echo -e "${GREEN}║  🛑 Стоп:     ./stop.sh                                  ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
echo ""

log "Мониторинг логов (Ctrl+C для выхода)..."
tail -f "$PROJECT_DIR/logs/backend.log" "$PROJECT_DIR/logs/frontend.log" 2>/dev/null
