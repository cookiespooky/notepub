import Link from "next/link";
import { ResetPasswordForm } from "@/components/ResetPasswordForm";
import styles from "../auth.module.css";

export default function ResetPasswordPage({ searchParams }: { searchParams: { token?: string } }) {
  const token = typeof searchParams.token === "string" ? searchParams.token : "";
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Сброс пароля</h1>
        <p className={styles.meta}>Перейдите по ссылке из письма и задайте новый пароль.</p>
        <ResetPasswordForm token={token} />
        <p className={styles.meta}>
          Вернуться к <Link href="/login">входу</Link>
        </p>
      </div>
    </main>
  );
}
