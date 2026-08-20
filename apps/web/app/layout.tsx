import type { Metadata } from "next";

import "./globals.css";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "pf-reliability",
  description: "Incident board and virtual metrics. Learning use only.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <div className="site-shell">
          <header className="site-header">
            <div className="site-brand">
              <a href="/" className="brand-link">
                <strong>pf-reliability</strong>
              </a>
              <span className="muted">P12 学習用インシデント管理</span>
              <p className="banner-warn">
                本番システムは操作しません。メトリクスは仮想合成です。自動 rollback / kubectl はありません。
              </p>
            </div>
            <nav className="site-nav">
              <a href="/">インシデント</a>
              <a href="/training">訓練採点</a>
            </nav>
          </header>
          <main className="site-main">{children}</main>
        </div>
      </body>
    </html>
  );
}
