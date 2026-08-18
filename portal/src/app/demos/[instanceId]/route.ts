import { NextRequest, NextResponse } from 'next/server';
import { getInstance } from '@/lib/firestore';

export const dynamic = 'force-dynamic';

export async function GET(
  req: NextRequest,
  { params }: { params: { instanceId: string } }
) {
  const instance = await getInstance(params.instanceId);
  if (instance?.status === 'RUNNING' && instance?.serviceUrl) {
    return NextResponse.redirect(instance.serviceUrl);
  }
  return NextResponse.json({ error: 'Demo instance not found or not yet ready.' }, { status: 404 });
}


