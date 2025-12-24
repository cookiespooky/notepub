import Link from "next/link";
import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { VerifyBanner } from "@/components/VerifyBanner";
import { UserMenu } from "@/components/UserMenu";
import styles from "./layout.module.css";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();
  if (!user) {
    redirect("/login");
  }

  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <div className={styles.headerInner}>
          <Link href="/dashboard/sites" className={styles.brand}>
            <div className={styles.avatar}>
              <img src='/logo.png' alt='Notepub' className={styles.avatarImage} />
            </div>
            <span>Notepub</span>
          </Link>
          <UserMenu email={user.email} />
        </div>
      </header>
      <VerifyBanner email={user.email} verified={user.emailVerified} />
      <main className={styles.main}>
        <div className={styles.content}>{children}</div>
      </main>
    </div>
  );
}
