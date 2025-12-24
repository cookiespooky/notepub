import Link from "next/link";
import styles from "./Breadcrumbs.module.css";

export type Crumb = {
  title: string;
  href: string | null;
};

type Props = {
  crumbs: Crumb[];
};

export function Breadcrumbs({ crumbs }: Props) {
  const needsRoot = !(crumbs[0]?.href === "/" || crumbs[0]?.title === "Главная");
  const items: Crumb[] = needsRoot ? [{ title: "Главная", href: "/" }, ...crumbs] : crumbs;
  return (
    <nav className={styles.trail} aria-label="breadcrumbs">
      {items.map((item, idx) => (
        <span key={`${item.title}-${idx}`} className={styles.item}>
          {item.href ? (
            <Link href={item.href} className={styles.link}>
              {item.title}
            </Link>
          ) : (
            <span className={styles.current}>{item.title}</span>
          )}
          {idx < items.length - 1 && <span className={styles.separator}>›</span>}
        </span>
      ))}
    </nav>
  );
}
