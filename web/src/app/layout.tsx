import type { Metadata } from "next";
import "./globals.css";
import { getMessages, DEFAULT_LOCALE } from "@/i18n";

const messages = getMessages(DEFAULT_LOCALE);

export const metadata: Metadata = {
  title: messages.appTitle,
  description: messages.appSubtitle,
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang={DEFAULT_LOCALE}>
      <body>{children}</body>
    </html>
  );
}
