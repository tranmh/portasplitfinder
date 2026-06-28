import { NextResponse } from "next/server";

// Used by the web container healthcheck (compose GETs /api/health).
export const dynamic = "force-dynamic";

export function GET() {
  return NextResponse.json(
    { status: "ok", time: new Date().toISOString() },
    { status: 200 },
  );
}
