"use client";

export default function ErrorPage({ reset }: { error: Error; reset: () => void }) {
  return (
    <main style={{ minHeight: "100vh", display: "grid", placeItems: "center", background: "#ffffff", color: "#222222" }}>
      <div style={{ textAlign: "center", padding: "24px" }}>
        <h1 style={{ margin: "0 0 8px 0" }}>Что-то пошло не так</h1>
        <p style={{ margin: "0 0 12px 0", color: "#4a5560" }}>
          Попробуйте обновить страницу.
        </p>
        <button
          onClick={reset}
          style={{
            padding: "10px 16px",
            borderRadius: 10,
            border: "1px solid #e0e0e0",
            background: "#0c4a6e",
            color: "#ffffff",
          }}
        >
          Обновить
        </button>
      </div>
    </main>
  );
}
