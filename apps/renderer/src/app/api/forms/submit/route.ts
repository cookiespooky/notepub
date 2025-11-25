import { NextResponse } from "next/server";
import { prisma } from "@notepub/db";
import { getSiteBySlug } from "@notepub/core";
import { sendLeadEmail } from "@/lib/mailer";

function extractSubdomain(host: string) {
  const withoutPort = host.split(":")[0] || "";
  const parts = withoutPort.split(".").filter(Boolean);
  if (parts.length === 2 && parts[1] === "localhost") {
    return parts[0];
  }
  if (parts.length <= 2) return "";
  return parts[0];
}

type FieldSchema = { name: string; label?: string; type: string; required?: boolean };

export async function POST(req: Request) {
  try {
    const host = req.headers.get("host") || "";
    const slug = req.headers.get("x-site-slug") || extractSubdomain(host);
    if (!slug) {
      return NextResponse.json({ message: "Сайт не найден" }, { status: 400 });
    }
    const site = await prisma.site.findUnique({ where: { slug }, include: { owner: true } });
    if (!site || !site.owner) {
      return NextResponse.json({ message: "Сайт не найден" }, { status: 404 });
    }
    if (!site.owner.emailVerified) {
      return NextResponse.json({ message: "Email владельца не подтвержден" }, { status: 400 });
    }

    const formData = await req.formData();
    const formId = (formData.get("formId") || "").toString().trim() || "form";
    const formTitle = (formData.get("formTitle") || "").toString().trim();
    const schemaRaw = formData.get("__schema")?.toString() || "";
    const pageUrl = formData.get("pageUrl")?.toString();

    let schema: { id?: string; title?: string; fields?: FieldSchema[] } = {};
    try {
      schema = schemaRaw ? JSON.parse(schemaRaw) : {};
    } catch {
      // ignore
    }
    const fields: FieldSchema[] = Array.isArray(schema.fields) ? schema.fields : [];

    const cleaned: Record<string, string> = {};
    for (const field of fields) {
      const name = typeof field.name === "string" ? field.name.trim() : "";
      const label = field.label || name;
      const type = field.type || "text";
      if (!name) continue;
      const raw = (formData.get(name) || "").toString();
      const value = raw.slice(0, 200).trim();
      const required = !!field.required;

      if (required && !value) {
        return NextResponse.json({ message: `Поле "${label}" обязательно` }, { status: 400 });
      }

      if (value) {
        if (type === "email" && !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(value)) {
          return NextResponse.json({ message: `Некорректный email в поле "${label}"` }, { status: 400 });
        }
        if (type === "phone" && !/^[0-9+\-\s()]{3,200}$/.test(value)) {
          return NextResponse.json({ message: `Некорректный телефон в поле "${label}"` }, { status: 400 });
        }
      }

      cleaned[name] = value;
    }

    await prisma.formSubmission.create({
      data: {
        siteId: site.id,
        ownerId: site.ownerId,
        formId,
        formTitle: formTitle || null,
        payload: { fields: cleaned, pageUrl },
      },
    });

    const lines = [
      `Форма: ${formTitle || formId}`,
      `Сайт: ${site.slug}.notepub.site`,
      pageUrl ? `Страница: ${pageUrl}` : null,
      "",
      ...Object.entries(cleaned).map(([key, val]) => `${key}: ${val || "(пусто)"}`),
    ].filter(Boolean) as string[];

    try {
      await sendLeadEmail(site.owner.email, `Новая заявка: ${formTitle || formId}`, lines.join("\n"));
    } catch (error) {
      console.error("Failed to send lead email", error);
    }

    return NextResponse.json({ message: "Отправлено" }, { status: 200 });
  } catch (error) {
    console.error("Form submit error", error);
    return NextResponse.json({ message: "Ошибка сервера" }, { status: 500 });
  }
}
