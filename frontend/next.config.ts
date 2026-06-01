import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',  // ← Нужно для Docker-сборки
  // Здесь могут быть другие настройки, если были
};

export default nextConfig;