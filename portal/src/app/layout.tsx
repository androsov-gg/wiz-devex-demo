import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Wiz DevEx Demo Platform",
  description: "Solution Engineering DevEx Demo Platform for Wiz",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="bg-slate-50 antialiased text-slate-900 min-h-screen">
        {children}
      </body>
    </html>
  );
}
