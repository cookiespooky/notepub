import Link from "next/link";
import { ForgotPasswordForm } from "@/components/ForgotPasswordForm";
import styles from "../auth.module.css";

export default function ForgotPasswordPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Восстановление пароля</h1>
        <p className={styles.meta}>Введите email — мы отправим код для сброса.</p>
        <ForgotPasswordForm />
        <p className={styles.meta}>
          Вспомнили пароль? <Link href="/login">Войти</Link>
        </p>
      </div>
    </main>
  );
}
