"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import styles from "./UserMenu.module.css";

export function UserMenu({ email }: { email: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      if (!ref.current) return;
      if (!ref.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, []);

  return (
    <div className={styles.menu} ref={ref}>
      <button
        type="button"
        className={styles.trigger}
        onClick={() => setOpen((prev) => !prev)}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <span className={styles.email}>{email}</span>
        <span className={open ? styles.caretOpen : styles.caret} aria-hidden />
      </button>
      {open && (
        <div className={styles.dropdown} role="menu">
          <Link href="/dashboard/account">Настройки аккаунта</Link>
          <Link href="/logout">Выйти</Link>
        </div>
      )}
    </div>
  );
}
