#!/bin/bash
set -e

# === Настройки ===
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$PROJECT_DIR/backend"
FRONTEND_DIR="$PROJECT_DIR/frontend"
LOG_DIR="$PROJECT_DIR/logs"
mkdir -p "$LOG_DIR"

# === Порты ===
BACKEND_PORT=8080
FRONTEND_PORT=3000
OLLAMA_PORT=11434

# === Цвета для вывода ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# === Логирование ===
log() { echo -e "${BLUE}[RAG]${NC} $1"; }
info() { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; }
step() { echo -e "${CYAN}➜${NC} $1"; }

# === Проверка, свободен ли порт ===
check_port() {
    local port=$1
    local name=$2
    if command -v lsof >/dev/null 2>&1; then
        if lsof -i :$port >/dev/null 2>&1; then
            local pid=$(lsof -t -i :$port)
            error "Порт $port занят (PID: $pid) — $name"
            return 1
        fi
    elif command -v ss >/dev/null 2>&1; then
        if ss -tlnp | grep -q ":$port "; then
            error "Порт $port занят — $name"
            return 1
        fi
    elif command -v netstat >/dev/null 2>&1; then
        if netstat -tlnp | grep -q ":$port "; then
            error "Порт $port занят — $name"
            return 1
        fi
    fi
    return 0
}

# === Освободить порт (убить процесс) ===
kill_port() {
    local port=$1
    local name=$2
    if command -v lsof >/dev/null 2>&1; then
        local pid=$(lsof -t -i :$port 2>/dev/null)
        if [ -n "$pid" ]; then
            warn "Освобождаю порт $port ($name), убиваю процесс $pid..."
            kill -9 $pid 2>/dev/null || true
            sleep 1
            return 0
        fi
    fi
    return 1
}

# === Интерактивное освобождение порта ===
free_port_interactive() {
    local port=$1
    local name=$2
    if ! check_port $port "$name"; then
        echo ""
        echo -e "${YELLOW}Порт $port занят. Что делать?${NC}"
        echo "  1) Освободить порт автоматически (убить процесс)"
        echo "  2) Выйти и освободить вручную"
        echo -n "Выбор [1/2]: "
        read -r choice
        if [[ "$choice" == "1" ]]; then
            if kill_port $port "$name"; then
                sleep 1
                if check_port $port "$name"; then
                    info "Порт $port освобождён ✅"
                    return 0
                fi
            fi
        fi
        error "Освободи порт $port вручную и запусти скрипт снова"
        return 1
    fi
    return 0
}

# === Очистка при выходе ===
cleanup() {
    log "Остановка сервисов..."
    [ -n "$OLLAMA_PID" ] && kill $OLLAMA_PID 2>/dev/null || true
    [ -n "$BACKEND_PID" ] && kill $BACKEND_PID 2>/dev/null || true
    [ -n "$FRONTEND_PID" ] && kill $FRONTEND_PID 2>/dev/null || true
    [ "$OLLAMA_STARTED_BY_US" = "true" ] && ollama serve stop 2>/dev/null || true
    info "Готово. До встречи! 👋"
    exit 0
}
trap cleanup SIGINT SIGTERM

# === Проверка зависимостей ===
log "Проверка зависимостей..."
command -v go >/dev/null 2>&1 || { error "Go не установлен"; exit 1; }
command -v node >/dev/null 2>&1 || { error "Node.js не установлен"; exit 1; }
command -v ollama >/dev/null 2>&1 || { error "Ollama не установлен: https://ollama.com"; exit 1; }
command -v psql >/dev/null 2>&1 || { error "PostgreSQL не установлен"; exit 1; }
info "Все зависимости найдены ✅"

# === Проверка моделей ===
log "Проверка ИИ-моделей..."
if ! ollama list 2>/dev/null | grep -q "nomic-embed-text"; then
    warn "Модель nomic-embed-text не найдена. Скачиваю..."
    ollama pull nomic-embed-text
fi
if ! ollama list 2>/dev/null | grep -q "mistral:7b"; then
    warn "Модель mistral:7b не найдена. Скачиваю..."
    ollama pull mistral:7b
fi
info "Модели готовы ✅"

# === Проверка/создание БД ===
log "Проверка базы данных..."
if ! sudo -u postgres psql -lqt 2>/dev/null | cut -d \| -f 1 | grep -qw rag_docs; then
    warn "База rag_docs не найдена. Создаю..."
    sudo -u postgres psql -c "CREATE DATABASE rag_docs;" 2>/dev/null || true
    sudo -u postgres psql -d rag_docs -c "CREATE EXTENSION IF NOT EXISTS vector;" 2>/dev/null || true
    info "База создана ✅"
else
    info "База данных найдена ✅"
fi

# === Проверка и освобождение портов ===
log "Проверка портов..."
free_port_interactive $BACKEND_PORT "Go-бэкенд" || exit 1
free_port_interactive $FRONTEND_PORT "Next.js-фронтенд" || exit 1
info "Порты готовы ✅"

# === Запуск Ollama (если не запущен) ===
if ! pgrep -f "ollama serve" >/dev/null; then
    step "Запуск Ollama..."
    ollama serve > "$LOG_DIR/ollama.log" 2>&1 &
    OLLAMA_PID=$!
    OLLAMA_STARTED_BY_US="true"
    sleep 3
    if ! kill -0 $OLLAMA_PID 2>/dev/null; then
        error "Ollama не запустился. Смотри лог: $LOG_DIR/ollama.log"
        exit 1
    fi
    info "Ollama запущен (PID: $OLLAMA_PID) ✅"
else
    info "Ollama уже работает ✅"
    OLLAMA_STARTED_BY_US="false"
fi

# === Запуск бэкенда ===
step "Запуск Go-бэкенда..."
cd "$BACKEND_DIR"
export DB_CONN="postgres://rag:ragpass@localhost:5432/rag_docs"
export OLLAMA_URL="http://localhost:11434"

go run cmd/server/main.go > "$LOG_DIR/backend.log" 2>&1 &
BACKEND_PID=$!
sleep 2

if ! kill -0 $BACKEND_PID 2>/dev/null; then
    error "Бэкенд не запустился! Смотри лог: $LOG_DIR/backend.log"
    tail -10 "$LOG_DIR/backend.log"
    exit 1
fi

for i in {1..5}; do
    if curl -s "http://localhost:$BACKEND_PORT/health" >/dev/null 2>&1; then
        info "Бэкенд отвечает на порту $BACKEND_PORT ✅"
        break
    fi
    if [ $i -eq 5 ]; then
        warn "Бэкенд не отвечает на порту $BACKEND_PORT (попробую ещё...)"
    fi
    sleep 1
done
info "Бэкенд запущен (PID: $BACKEND_PID) ✅"

# === Запуск фронтенда ===
step "Запуск Next.js-фронтенда..."
cd "$FRONTEND_DIR"

# 🔧 Чистим кэш, чтобы избежать ошибок "required-server-files.json"
warn "Очистка кэша Next.js..."
rm -rf .next

# 🔧 Гарантируем установку зависимостей (локально, а не глобально!)
if [ ! -d "node_modules" ]; then
    step "Установка зависимостей npm (это может занять время)..."
    npm install --silent
fi

export RAG_API_URL="http://localhost:$BACKEND_PORT/api/search"
export OLLAMA_URL="http://localhost:11434/api/generate"

# Запускаем dev-сервер
npm run dev > "$LOG_DIR/frontend.log" 2>&1 &
FRONTEND_PID=$!

# 🔍 Ждём, пока фронтенд ДЕЙСТВИТЕЛЬНО начнёт отвечать на запросы
step "Ожидание готовности фронтенда..."
for i in {1..30}; do  # 30 попыток по 1 сек = макс 30 сек ожидания
    if curl -s "http://localhost:$FRONTEND_PORT" >/dev/null 2>&1; then
        info "Фронтенд отвечает на порту $FRONTEND_PORT ✅"
        break
    fi
    if [ $i -eq 30 ]; then
        error "Фронтенд не запустился за 30 секунд! Смотри лог:"
        tail -20 "$LOG_DIR/frontend.log"
        cleanup
    fi
    sleep 1
done

info "Фронтенд запущен (PID: $FRONTEND_PID) ✅"

# === Финальное сообщение ===
echo ""
echo -e "${GREEN}╔════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  🚀 RAG Portfolio запущен!        ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  🌐 Открой: ${NC}http://localhost:$FRONTEND_PORT   ${GREEN}║${NC}"
echo -e "${GREEN}║  🔌 Backend: ${NC}http://localhost:$BACKEND_PORT  ${GREEN}║${NC}"
echo -e "${GREEN}║  🧠 Ollama:  ${NC}http://localhost:$OLLAMA_PORT ${GREEN}║${NC}"
echo -e "${GREEN}╠════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  📋 Логи:                          ║${NC}"
echo -e "${GREEN}║  • backend:  $LOG_DIR/backend.log  ║${NC}"
echo -e "${GREEN}║  • frontend: $LOG_DIR/frontend.log ║${NC}"
echo -e "${GREEN}║  • ollama:   $LOG_DIR/ollama.log   ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  💡 Нажми ${NC}Ctrl+C${GREEN} для остановки  ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════╝${NC}"
echo ""

# === Мониторинг процессов ===
log "Мониторинг сервисов (нажми Ctrl+C для остановки)..."
while kill -0 $BACKEND_PID $FRONTEND_PID 2>/dev/null; do
    sleep 5
    if ! kill -0 $BACKEND_PID 2>/dev/null; then
        error "Бэкенд упал! Смотри лог: $LOG_DIR/backend.log"
        tail -20 "$LOG_DIR/backend.log"
        cleanup
    fi
    if ! kill -0 $FRONTEND_PID 2>/dev/null; then
        error "Фронтенд упал! Смотри лог: $LOG_DIR/frontend.log"
        tail -20 "$LOG_DIR/frontend.log"
        cleanup
    fi
done

cleanup
