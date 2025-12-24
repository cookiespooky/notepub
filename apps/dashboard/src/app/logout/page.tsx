import { logoutAction } from "./actions";
import styles from "../auth.module.css";

export default function LogoutPage() {
  return (
    <main className={styles.shell}>
      <div className={styles.card}>
        <h1 className={styles.title}>Выйти из аккаунта?</h1>
        <form action={logoutAction} className={styles.form}>
          <button type="submit" className={styles.primary}>
            Выйти
          </button>
        </form>
      </div>
    </main>
  );
}
