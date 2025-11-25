"use client";

import { useEffect } from "react";

export function FormHandler() {
  useEffect(() => {
    const forms = Array.from(document.querySelectorAll<HTMLFormElement>("[data-notepub-form]"));
    const cleanup: Array<() => void> = [];

    forms.forEach((form) => {
      const handler = async (event: Event) => {
        event.preventDefault();
        const status = form.querySelector<HTMLElement>(".np-form-status");
        const submitButton = form.querySelector<HTMLButtonElement>('button[type="submit"]');
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
            status.textContent = message;
            status.dataset.state = res.ok ? "success" : "error";
          } else {
            alert(message);
          }
          if (res.ok) form.reset();
        } catch (error) {
          if (status) {
            status.textContent = "Не удалось отправить форму";
            status.dataset.state = "error";
          } else {
            alert("Не удалось отправить форму");
          }
        } finally {
          if (submitButton) submitButton.disabled = false;
        }
      };
      form.addEventListener("submit", handler);
      cleanup.push(() => form.removeEventListener("submit", handler));
    });

    return () => cleanup.forEach((fn) => fn());
  }, []);

  return null;
}
