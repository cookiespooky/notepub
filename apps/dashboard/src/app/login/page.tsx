import Link from "next/link";
import { UnifiedAuthForm } from "@/components/UnifiedAuthForm";
import styles from "../auth.module.css";

export default function LoginPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Войти</h1>
        <UnifiedAuthForm />
      </div>
    </main>
  );
}
