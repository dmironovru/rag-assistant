# RAG Assistant — умный AI-ассистент с поиском по документам

## 🚀 Быстрый старт

### Требования:
- Docker и Docker Compose
- Ollama (для работы LLM)

### Установка и запуск:

```bash
# 1. Клонируй репозиторий
git clone https://github.com/dmironovru/rag-assistant.git
cd rag-assistant

# 2. Установи русскоязычную модель Ollama
ollama pull cyberlis/saiga-mistral:7b-lora-q4_K

# 3. Запусти всё одной командой
docker-compose up -d

# 4. Открой браузер: http://localhost:3000
