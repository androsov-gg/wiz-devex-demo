import { NextRequest, NextResponse } from 'next/server';
import { getDemoServiceLogs } from '@/lib/cloudrun';

export async function GET(
  req: NextRequest,
  { params }: { params: { id: string } }
) {
  try {
    const instanceId = params.id;
    const linesParam = req.nextUrl.searchParams.get('lines');
    const lines = linesParam ? parseInt(linesParam, 10) : 100;

    const logs = await getDemoServiceLogs(instanceId, lines);
    return NextResponse.json({ logs });
  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}

