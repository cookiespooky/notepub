import { NextResponse } from "next/server";
import { getIndexData } from "@/lib/notes";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const data = await getIndexData();
    // hide storage key from public response
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const flat = data.flat.map(({ key: _key, ...rest }) => rest);
    return NextResponse.json({ categories: data.categories, flat });
  } catch (error: unknown) {
    console.error("Failed to build index", error);
    return NextResponse.json({ error: "Failed to load index" }, { status: 500 });
  }
}
