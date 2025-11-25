"use server";

import { redirect } from "next/navigation";
import { isRedirectError } from "next/dist/client/components/redirect";
import { registerUser, isUniqueConstraintError } from "@/lib/auth";
import { setSession } from "@/lib/session";
import { issueVerificationCode } from "@/lib/codes";
import { sendVerificationEmail } from "@/lib/emails";

export async function signupAction(_: { error?: string } | undefined, formData: FormData) {
  const email = (formData.get("email") || "").toString().trim().toLowerCase();
  const password = (formData.get("password") || "").toString();

  if (!email || !password) {
    return { error: "Email и пароль обязательны" };
  }
  if (password.length < 6) {
    return { error: "Пароль должен быть не короче 6 символов" };
  }
  try {
    const user = await registerUser(email, password);
    setSession(user.id, false);
    // Try to send verification code, but don't block signup if email fails.
    issueVerificationCode(user.id)
      .then((code) => sendVerificationEmail(email, code))
      .catch((error) => console.error("Failed to send verification email", error));
  } catch (err) {
    if (isRedirectError(err)) throw err;
    if (isUniqueConstraintError(err, "email")) {
      return { error: "Email уже используется" };
    }
    return { error: "Не удалось создать аккаунт" };
  }
  redirect("/dashboard/sites");
}
