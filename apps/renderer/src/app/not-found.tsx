export default function NotFound() {
  return (
    <main style={{ minHeight: "100vh", display: "grid", placeItems: "center", background: "#ffffff", color: "#222222" }}>
      <div style={{ textAlign: "center", padding: "24px" }}>
        <h1 style={{ margin: "0 0 8px 0" }}>Страница не найдена</h1>
        <p style={{ margin: 0, color: "#4a5560" }}>Проверьте путь или вернитесь на главную страницу сайта.</p>
      </div>
    </main>
  );
}
