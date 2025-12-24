import styles from "./Breadcrumbs.module.css";
import { NavLink } from "@/components/NavContext";
import HomeRoundedIcon from "@mui/icons-material/HomeRounded";

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

  const renderLabel = (item: Crumb) =>
    item.title === "Главная" && item.href === "/" ? (
      <span className={styles.homeIcon} aria-label="Главная">
        <HomeRoundedIcon style={{fontSize: 18, lineHeight: 1}} />
      </span>
    ) : (
      item.title
    );

  return (
    <nav className={styles.trail} aria-label="breadcrumbs">
      {items.map((item, idx) => (
        <span key={`${item.title}-${idx}`} className={styles.item}>
          {item.href ? (
            <NavLink href={item.href} className={styles.link}>
              {renderLabel(item)}
            </NavLink>
          ) : (
            <span className={styles.current}>{renderLabel(item)}</span>
          )}
          {idx < items.length - 1 && <span className={styles.separator}>›</span>}
        </span>
      ))}
    </nav>
  );
}
