import Link from "next/link";
import { ResetPasswordForm } from "@/components/ResetPasswordForm";
import styles from "../auth.module.css";

export default function ResetPasswordPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Сброс пароля</h1>
        <p className={styles.meta}>Введите код из письма и новый пароль.</p>
        <ResetPasswordForm />
        <p className={styles.meta}>
          Вернуться к <Link href="/login">входу</Link>
        </p>
      </div>
    </main>
  );
}
