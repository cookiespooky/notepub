"use server";

import { redirect } from "next/navigation";
import { authenticate } from "@/lib/auth";
import { setSession } from "@/lib/session";
import { issueVerificationCode } from "@/lib/codes";
import { sendVerificationEmail } from "@/lib/emails";

export async function loginAction(_: { error?: string } | undefined, formData: FormData) {
  const email = (formData.get("email") || "").toString().trim().toLowerCase();
  const password = (formData.get("password") || "").toString();

  if (!email || !password) {
    return { error: "Email и пароль обязательны" };
  }

  const user = await authenticate(email, password);
  if (!user) {
    return { error: "Неверные логин или пароль" };
  }

  setSession(user.id, user.emailVerified);
  if (!user.emailVerified) {
    // Fire and forget verification email; don't block login.
    issueVerificationCode(user.id)
      .then((code) => sendVerificationEmail(email, code))
      .catch((error) => console.error("Failed to send verification email", error));
  }
  redirect("/dashboard/sites");
}
