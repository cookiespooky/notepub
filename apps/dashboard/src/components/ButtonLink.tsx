"use client";

import Link from "next/link";
import styles from "../app/dashboard/sites/sites.module.css";

type ButtonLinkProps = {
  href: string;
  children: React.ReactNode;
  variant?: "primary" | "secondary";
  target?: string;
};

export function ButtonLink({ href, children, variant = "primary", target }: ButtonLinkProps) {
  const className = variant === "secondary" ? styles.secondary : styles.primary;
  return (
    <Link href={href} className={className} target={target}>
      {children}
    </Link>
  );
}
