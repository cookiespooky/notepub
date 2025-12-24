import Link from "next/link";
import { SignupForm } from "@/components/SignupForm";
import styles from "../auth.module.css";

export default function SignupPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Создать аккаунт</h1>
        <SignupForm />
        <p className={styles.meta}>
          Уже есть аккаунт? <Link href="/login">Войти</Link>
        </p>
      </div>
    </main>
  );
}
