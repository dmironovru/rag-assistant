import type { Metadata } from "next";
import { Manrope } from "next/font/google";
import "./globals.css";

const manrope = Manrope({
  subsets: ["latin", "cyrillic"],
  variable: "--font-manrope",
  display: "swap",
});

export const metadata: Metadata = {
  title: "RAG Ассистент | Дмитрий Миронов",
  description: "Умный ассистент на основе RAG (Retrieval-Augmented Generation)",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" className={manrope.variable}>
      <body className={`${manrope.className} antialiased bg-[#0a0a0a] text-white`}>
        <main className="min-h-screen pt-16">
          {children}
        </main>
      </body>
    </html>
  );
}