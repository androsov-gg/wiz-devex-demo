import { NextRequest, NextResponse } from 'next/server';
import { restartDemoService } from '@/lib/cloudrun';
import { getInstance, updateInstanceStatus } from '@/lib/firestore';

export async function POST(
  req: NextRequest,
  { params }: { params: { id: string } }
) {
  try {
    const instanceId = params.id;
    const instance = await getInstance(instanceId);
    // Immediately mark status as RESTARTING and reset cloudRunReadyAt
    await updateInstanceStatus(instanceId, 'RESTARTING', { cloudRunReadyAt: '' });

    // Trigger Cloud Run restart rollout with IAP preserved
    restartDemoService(instanceId, instance?.ownerEmail)
      .then(async () => {
        console.log(`Cloud Run restart completed for ${instanceId}`);
      })
      .catch(async (err: any) => {
        console.error(`Restart failed for ${instanceId}:`, err);
        await updateInstanceStatus(instanceId, 'FAILED', {
          errorMessage: `Restart failed: ${err?.message || err}`,
        });
      });

    return NextResponse.json({ message: `Instance ${instanceId} restart initiated.`, status: 'RESTARTING' });
  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}

