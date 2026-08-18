import { NextRequest, NextResponse } from 'next/server';
import { getDemoCatalog } from '@/lib/catalog';

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  const forceRefresh = req.nextUrl.searchParams.get('refresh') === 'true';
  const catalog = await getDemoCatalog(forceRefresh);
  return NextResponse.json({ catalog });
}
