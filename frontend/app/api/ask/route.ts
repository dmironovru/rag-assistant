// app/api/ask/route.ts
import { NextResponse } from 'next/server'

const RAG_API = process.env.RAG_API_URL || 'http://localhost:8080/api/search'
const OLLAMA_URL = process.env.OLLAMA_URL || 'http://localhost:11434/api/generate'
const LLM_MODEL = process.env.LLM_MODEL || 'mistral:7b' // или 'llama3:8b'

export async function POST(req: Request) {
  try {
    const { query } = await req.json()
    if (!query?.trim()) {
      return NextResponse.json({ error: 'Query is required' }, { status: 400 })
    }

    // 1. Получаем релевантные чанки из Go-бэкенда
    const searchRes = await fetch(RAG_API, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, limit: 5 }),
    })
    if (!searchRes.ok) throw new Error('RAG API error')
    const chunks = await searchRes.json() as Array<{ content: string; source: string; similarity: number }>

    // 2. Формируем контекст для LLM
    const contextText = chunks
      .map((c, i) => `[${i + 1}] ${c.content.trim()}`)
      .join('\n\n')

    // 3. Промпт для генерации ответа
    const prompt = `Ты — помощник разработчика. Отвечай на вопрос, используя ТОЛЬКО предоставленный контекст.
Если в контексте нет ответа — честно скажи "В загруженной документации нет информации по этому вопросу".
Не выдумывай факты. Цитируй источники в формате [1], [2].

Контекст:
${contextText}

Вопрос: ${query}

Ответ:`

    // 4. Запрашиваем ответ у LLM
    const llmRes = await fetch(OLLAMA_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: LLM_MODEL,
        prompt,
        stream: false,
        options: { temperature: 0.3, num_predict: 512 }
      }),
    })
    if (!llmRes.ok) throw new Error('LLM API error')
    const llmData = await llmRes.json()

    // 5. Формируем ответ для фронтенда
    return NextResponse.json({
      answer: llmData.response.trim(),
      context: chunks.map(c => c.content),
      sources: chunks.map(c => ({ source: c.source, similarity: c.similarity })),
    })

  } catch (error) {
    console.error('RAG ask error:', error)
    return NextResponse.json(
      { error: 'Failed to generate answer', details: error instanceof Error ? error.message : 'Unknown' },
      { status: 500 }
    )
  }
}