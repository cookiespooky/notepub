import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { VerifyEmailForm } from "@/components/VerifyEmailForm";
import styles from "../auth.module.css";

export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams?: { token?: string; error?: string };
}) {
  const user = await getCurrentUser();
  if (!user) {
    redirect("/login");
  }
  if (user.emailVerified) {
    redirect("/dashboard/sites");
  }

  const token = searchParams?.token;
  if (token) {
    redirect(`/verify-email/confirm?token=${encodeURIComponent(token)}`);
  }

  const error = searchParams?.error;

  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Подтверждение email</h1>
        {error && <p className={styles.error}>{error}</p>}
        <p className={styles.meta}>Мы отправили письмо на {user.email}. Перейдите по ссылке из письма или введите код, если он есть.</p>
        <VerifyEmailForm email={user.email} />
      </div>
    </main>
  );
}
