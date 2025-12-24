"use client";

import { useRouter } from "next/navigation";

type Props = {
  fallbackHref: string;
  className?: string;
  children: React.ReactNode;
};

export function BackButton({ fallbackHref, className, children }: Props) {
  const router = useRouter();

  const handleClick = (event: React.MouseEvent<HTMLAnchorElement | HTMLButtonElement>) => {
    event.preventDefault();
    if (window.history.length > 1) {
      router.back();
    } else {
      router.push(fallbackHref);
    }
  };

  return (
    <a href={fallbackHref} className={className} style={{width: 'fit-content'}} onClick={handleClick}>
      {children}
    </a>
  );
}
