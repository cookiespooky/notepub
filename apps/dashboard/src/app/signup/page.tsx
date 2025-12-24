import Link from "next/link";
import { UnifiedAuthForm } from "@/components/UnifiedAuthForm";
import styles from "../auth.module.css";

export default function SignupPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Создать аккаунт</h1>
        <UnifiedAuthForm />
      </div>
    </main>
  );
}
