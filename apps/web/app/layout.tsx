import type { Metadata } from "next";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "pf-reliability",
  description: "Incident board and virtual metrics. Learning use only.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body style={{ fontFamily: "system-ui, sans-serif", margin: "1.5rem", maxWidth: 1100, lineHeight: 1.5 }}>
        <header style={{ marginBottom: "1.5rem", borderBottom: "1px solid #ddd", paddingBottom: "0.75rem" }}>
          <a href="/" style={{ textDecoration: "none", color: "inherit" }}>
            <strong>pf-reliability</strong>
          </a>
          <span style={{ color: "#666", marginLeft: "0.75rem", fontSize: "0.9rem" }}>P12 学習用インシデント管理</span>
          <p
            style={{
              margin: "0.75rem 0 0",
              padding: "0.5rem 0.75rem",
              background: "#fff4e5",
              border: "1px solid #f0c36d",
              borderRadius: 6,
            }}
          >
            本番システムは操作しません。メトリクスは仮想合成です。自動 rollback / kubectl はありません。
          </p>
          <nav style={{ marginTop: "0.75rem" }}>
            <a href="/">インシデント</a>
            {" · "}
            <a href="/training">訓練採点</a>
          </nav>
        </header>
        {children}
      </body>
    </html>
  );
}
