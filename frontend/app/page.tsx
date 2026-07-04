'use client';

import { useState, useRef, useEffect } from 'react';
import { Send, Loader2, Sparkles, Upload, CheckCircle, FileText } from 'lucide-react';

interface Message {
  role: 'user' | 'assistant';
  content: string;
  context?: string[];
}

interface UploadProgress {
  fileName: string;
  progress: number;
  status: 'idle' | 'uploading' | 'processing' | 'complete' | 'error';
  chunks?: number;
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
  const [uploadProgress, setUploadProgress] = useState<UploadProgress>({
    fileName: '',
    progress: 0,
    status: 'idle',
  });
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

    // Начинаем загрузку
    setIsUploading(true);
    setUploadProgress({
      fileName: file.name,
      progress: 0,
      status: 'uploading',
    });

    const formData = new FormData();
    formData.append('file', file);

    // Эмулируем прогресс загрузки (пока бэкенд не умеет отправлять прогресс)
    const progressInterval = setInterval(() => {
      setUploadProgress(prev => {
        const newProgress = Math.min(prev.progress + 5, 95);
        return {
          ...prev,
          progress: newProgress,
        };
      });
    }, 200);

    try {
      const response = await fetch(`${BACKEND_URL}/api/upload`, {
        method: 'POST',
        body: formData,
      });

      clearInterval(progressInterval);

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      const data = await response.json();
      
      const chunkCount = data.chunks ?? 0;
      const filename = data.filename || file.name;

      // Завершаем с успехом
      setUploadProgress({
        fileName: filename,
        progress: 100,
        status: 'complete',
        chunks: chunkCount,
      });

      // Добавляем сообщение в чат
      setTimeout(() => {
        setMessages(prev => [...prev, {
          role: 'assistant',
          content: `✅ Файл "${filename}" загружен и проиндексирован (${chunkCount} чанков). Теперь можно задавать вопросы по его содержимому!`,
        }]);
        setUploadProgress({
          fileName: '',
          progress: 0,
          status: 'idle',
        });
        setIsUploading(false);
      }, 800);

    } catch (error) {
      clearInterval(progressInterval);
      console.error('Upload error:', error);
      
      setUploadProgress({
        fileName: file.name,
        progress: 0,
        status: 'error',
      });

      setMessages(prev => [...prev, {
        role: 'assistant',
        content: `❌ Ошибка загрузки файла: ${error instanceof Error ? error.message : 'Неизвестная ошибка'}`,
      }]);

      setTimeout(() => {
        setUploadProgress({
          fileName: '',
          progress: 0,
          status: 'idle',
        });
        setIsUploading(false);
      }, 3000);
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
            className="ml-auto px-4 py-2 rounded-lg bg-purple-900/50 text-purple-200 text-sm hover:bg-purple-900/70 transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isUploading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Загрузка...
              </>
            ) : (
              <>
                <Upload className="w-4 h-4" />
                Загрузить файл
              </>
            )}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".txt,.md,.go,.pdf,.docx,.doc,.json,.yaml,.yml,.csv,.xml"
            onChange={handleFileUpload}
            className="hidden"
          />
        </div>

        {/* Индикатор загрузки */}
        {uploadProgress.status !== 'idle' && (
          <div className="mb-4 p-4 rounded-lg bg-white/5 border border-white/10">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <FileText className="w-4 h-4 text-purple-400" />
                <span className="text-sm text-gray-300 truncate max-w-[200px]">
                  {uploadProgress.fileName}
                </span>
              </div>
              <span className="text-sm text-gray-400">
                {uploadProgress.status === 'uploading' && `${Math.round(uploadProgress.progress)}%`}
                {uploadProgress.status === 'processing' && '⏳ Индексация...'}
                {uploadProgress.status === 'complete' && '✅ Готово!'}
                {uploadProgress.status === 'error' && '❌ Ошибка'}
              </span>
            </div>
            <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-300 rounded-full ${
                  uploadProgress.status === 'error'
                    ? 'bg-red-500'
                    : uploadProgress.status === 'complete'
                    ? 'bg-green-500'
                    : 'bg-purple-500'
                }`}
                style={{ width: `${Math.min(uploadProgress.progress, 100)}%` }}
              />
            </div>
            {uploadProgress.status === 'complete' && uploadProgress.chunks !== undefined && (
              <div className="mt-1 text-xs text-gray-400">
                {uploadProgress.chunks} чанков проиндексировано
              </div>
            )}
          </div>
        )}

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
            disabled={isLoading}
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