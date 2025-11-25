import Link from "next/link";
import { LoginForm } from "@/components/LoginForm";
import styles from "../auth.module.css";

export default function LoginPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Войти</h1>
        <LoginForm />
        <p className={styles.meta}>
          Нет аккаунта? <Link href="/signup">Создать</Link>
        </p>
      </div>
    </main>
  );
}
