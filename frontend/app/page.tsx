'use client';

import { useState, useRef, useEffect } from 'react';
import { Send, Loader2, Sparkles } from 'lucide-react';

interface Message {
  role: 'user' | 'assistant';
  content: string;
  context?: string[];
}

const BACKEND_URL = process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080';

export default function Home() {
  const [messages, setMessages] = useState<Message[]>([
    {
      role: 'assistant',
      content: 'Привет! Я RAG-ассистент. Загрузи документы или задай вопрос, и я отвечу, используя базу знаний.',
    },
  ]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const sendMessage = async () => {
    if (!input.trim() || isLoading) return;

    const userMessage: Message = { role: 'user', content: input };
    setMessages(prev => [...prev, userMessage]);
    setInput('');
    setIsLoading(true);

    try {
      const response = await fetch(`${BACKEND_URL}/api/ask`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ question: input }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      const data = await response.json();
      
      const sources = data.sources || [];
      const contextStrings = sources.map((source: any) => {
        if (typeof source === 'string') return source;
        if (source && typeof source === 'object') {
          return source.source || source.path || source.filename || JSON.stringify(source);
        }
        return String(source);
      });

      const assistantMessage: Message = {
        role: 'assistant',
        content: data.answer || 'Не удалось получить ответ',
        context: contextStrings,
      };
      setMessages(prev => [...prev, assistantMessage]);
    } catch (error) {
      console.error('Error:', error);
      setMessages(prev => [...prev, {
        role: 'assistant',
        content: `❌ Ошибка: ${error instanceof Error ? error.message : 'Неизвестная ошибка'}`,
      }]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setIsUploading(true);
    const formData = new FormData();
    formData.append('file', file);

    try {
      const response = await fetch(`${BACKEND_URL}/api/upload`, {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      const data = await response.json();
      
      const chunkCount = data.chunks ?? 0;
      const filename = data.filename || file.name;

      setMessages(prev => [...prev, {
        role: 'assistant',
        content: `✅ Файл "${filename}" загружен и проиндексирован (${chunkCount} чанков). Теперь можно задавать вопросы по его содержимому!`,
      }]);
    } catch (error) {
      console.error('Upload error:', error);
      setMessages(prev => [...prev, {
        role: 'assistant',
        content: `❌ Ошибка загрузки файла: ${error instanceof Error ? error.message : 'Неизвестная ошибка'}`,
      }]);
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <div className="glass-card p-6">
        <div className="flex items-center gap-3 mb-6">
          <Sparkles className="w-8 h-8 text-purple-400" />
          <h1 className="text-2xl md:text-3xl font-bold">RAG Ассистент</h1>
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={isUploading}
            className="ml-auto px-4 py-2 rounded-lg bg-purple-900/50 text-purple-200 text-sm hover:bg-purple-900/70 transition disabled:opacity-50"
          >
            {isUploading ? 'Загрузка...' : '📄 Загрузить файл'}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".txt,.md,.go,.pdf,.docx,.doc,.json,.yaml,.yml,.csv,.xml"
            onChange={handleFileUpload}
            className="hidden"
          />
        </div>

        <div className="space-y-4 mb-4 max-h-[60vh] overflow-y-auto">
          {messages.map((message, idx) => (
            <div
              key={idx}
              className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-[80%] rounded-2xl px-4 py-3 ${
                  message.role === 'user'
                    ? 'bg-purple-900/50 text-white'
                    : 'bg-white/5 text-gray-200'
                }`}
              >
                <div className="whitespace-pre-wrap">{message.content}</div>
                {message.context && message.context.length > 0 && (
                  <details className="mt-2 text-xs text-gray-400">
                    <summary>Источники ({message.context.length})</summary>
                    <ul className="mt-1 pl-4 space-y-1">
                      {message.context.map((ctx, i) => (
                        <li key={i} className="break-all">{ctx}</li>
                      ))}
                    </ul>
                  </details>
                )}
              </div>
            </div>
          ))}
          {isLoading && (
            <div className="flex justify-start">
              <div className="bg-white/5 rounded-2xl px-4 py-3 flex items-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span className="text-gray-400">Думаю...</span>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        <div className="flex gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyPress={handleKeyPress}
            placeholder="Задайте вопрос..."
            className="flex-1 bg-white/5 border border-white/10 rounded-xl px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-purple-500 resize-none"
            rows={2}
          />
          <button
            onClick={sendMessage}
            disabled={isLoading || !input.trim()}
            className="px-4 py-2 rounded-xl bg-purple-900/50 text-purple-200 hover:bg-purple-900/70 transition disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Send className="w-5 h-5" />
          </button>
        </div>
      </div>
    </div>
  );
}
