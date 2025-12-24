import { NextRequest, NextResponse } from "next/server";
import { loadEnv } from "@notepub/env";
import { verifyEmailByToken } from "../actions";

export async function GET(request: NextRequest) {
  const url = new URL(request.url);
  const env = loadEnv();
  const appUrl = env.APP_URL || `${url.protocol}//${url.host}`;
  const token = url.searchParams.get("token");

  if (!token) {
    const redirectUrl = new URL("/verify-email", appUrl);
    redirectUrl.searchParams.set("error", "Некорректная ссылка подтверждения.");
    return NextResponse.redirect(redirectUrl);
  }

  const result = await verifyEmailByToken(token);

  if (result.success) {
    return NextResponse.redirect(new URL("/dashboard/sites", appUrl));
  }

  if (result.error === "Нужно войти, чтобы подтвердить email.") {
    return NextResponse.redirect(new URL("/login", appUrl));
  }

  const redirectUrl = new URL("/verify-email", appUrl);
  redirectUrl.searchParams.set("error", result.error || "Не удалось подтвердить ссылку.");
  return NextResponse.redirect(redirectUrl);
}
