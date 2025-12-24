"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";

export function FormHandler() {
  const pathname = usePathname();

  useEffect(() => {
    const forms = Array.from(document.querySelectorAll<HTMLFormElement>("[data-notepub-form]"));
    const cleanup: Array<() => void> = [];
    const timers: NodeJS.Timeout[] = [];

    forms.forEach((form) => {
      const handler = async (event: Event) => {
        event.preventDefault();
        const status = form.querySelector<HTMLElement>(".np-form-status");
        const title = form.querySelector<HTMLElement>(".np-form-title")?.textContent?.trim() || "";
        const submitButton = form.querySelector<HTMLButtonElement>('button[type="submit"]');
        const redirectInput = form.querySelector<HTMLInputElement>('input[name="__redirect"]');
        const redirectUrl = redirectInput?.value?.trim() || "";
        if (status) {
          status.textContent = "";
          status.dataset.state = "";
        }
        if (submitButton) submitButton.disabled = true;
        try {
          const formData = new FormData(form);
          formData.set("pageUrl", window.location.href);
          const res = await fetch("/api/forms/submit", {
            method: "POST",
            body: formData,
          });
          const data = await res.json().catch(() => null);
          const message = data?.message || (res.ok ? "Отправлено" : "Ошибка отправки");
          if (status) {
            status.innerHTML = title ? `<div class="np-status-title">${title}</div><div>${message}</div>` : message;
            status.dataset.state = res.ok ? "success" : "error";
          } else {
            alert(message);
          }
          if (res.ok) {
            form.reset();
            form.classList.add("np-status-visible");
            const timer = setTimeout(() => {
              form.classList.remove("np-status-visible");
              if (redirectUrl && isHttpUrl(redirectUrl)) {
                window.location.href = redirectUrl;
              }
            }, 2000);
            timers.push(timer);
          }
        } catch (error) {
          if (status) {
            status.textContent = "Не удалось отправить форму";
            status.dataset.state = "error";
          } else {
            alert("Не удалось отправить форму");
          }
          form.classList.remove("np-status-visible");
        } finally {
          if (submitButton) submitButton.disabled = false;
        }
      };
      form.addEventListener("submit", handler);
      cleanup.push(() => form.removeEventListener("submit", handler));
    });

    return () => {
      cleanup.forEach((fn) => fn());
      timers.forEach((t) => clearTimeout(t));
    };
  }, [pathname]);

  return null;
}

function isHttpUrl(candidate: string) {
  try {
    const url = new URL(candidate);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}
