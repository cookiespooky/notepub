import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { VerifyEmailForm } from "@/components/VerifyEmailForm";
import { verifyEmailByToken } from "./actions";
import styles from "../auth.module.css";

export default async function VerifyEmailPage({ searchParams }: { searchParams?: { token?: string } }) {
  const user = await getCurrentUser();
  if (!user) {
    redirect("/login");
  }
  if (user.emailVerified) {
    redirect("/dashboard/sites");
  }

  const token = searchParams?.token;
  if (token) {
    const result = await verifyEmailByToken(token);
    if (result.success) {
      redirect("/dashboard/sites");
    }
  }

  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Подтверждение email</h1>
        <p className={styles.meta}>Мы отправили письмо на {user.email}. Перейдите по ссылке из письма или введите код, если он есть.</p>
        <VerifyEmailForm email={user.email} />
      </div>
    </main>
  );
}
