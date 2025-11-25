import Link from "next/link";
import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { VerifyBanner } from "@/components/VerifyBanner";
import styles from "./layout.module.css";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();
  if (!user) {
    redirect("/login");
  }

  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <div className={styles.brand}>
          <Link href="/dashboard/sites">Notepub</Link>
        </div>
        <nav className={styles.nav}>
          <Link href="/dashboard/sites">Sites</Link>
          <Link href="/logout">Logout</Link>
        </nav>
        <div className={styles.user}>{user.email}</div>
      </header>
      <VerifyBanner email={user.email} verified={user.emailVerified} />
      <main className={styles.main}>{children}</main>
    </div>
  );
}
