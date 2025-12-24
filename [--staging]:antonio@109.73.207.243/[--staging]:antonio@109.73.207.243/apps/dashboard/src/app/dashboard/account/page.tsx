import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { EmailForm, PasswordForm } from "./AccountForms";
import styles from "./account.module.css";
import { BackButton } from "./BackButton";

export default async function AccountPage() {
  const user = await getCurrentUser();
  if (!user) {
    redirect("/login");
  }

  return (
    <div className={styles.shell}>
      <div className={styles.header}>
        <div>
          <h1>Управление доступом</h1>
        </div>
        <BackButton className={styles.primary} fallbackHref="/dashboard/sites">
          Назад
        </BackButton>
      </div>

      <div className={styles.grid}>
        <EmailForm currentEmail={user.email} />
        <PasswordForm />
      </div>
    </div>
  );
}
