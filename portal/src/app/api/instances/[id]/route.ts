import { NextRequest, NextResponse } from 'next/server';
import { getInstance, updateInstanceStatus, deleteInstanceRecord } from '@/lib/firestore';
import { deleteDemoService, getDemoServiceStatus } from '@/lib/cloudrun';

const IAP_PROPAGATION_DELAY_MS = 90 * 1000;

export async function GET(
  req: NextRequest,
  { params }: { params: { id: string } }
) {
  try {
    const instanceId = params.id;
    const instance = await getInstance(instanceId);

    if (!instance) {
      return NextResponse.json({ error: 'Instance not found.' }, { status: 404 });
    }

    if (instance.status === 'RUNNING' || instance.status === 'PROVISIONING' || instance.status === 'RESTARTING') {
      const { status: liveStatus, serviceUrl } = await getDemoServiceStatus(instanceId);

      if (liveStatus === 'FAILED') {
        await updateInstanceStatus(instanceId, 'FAILED', { serviceUrl });
        return NextResponse.json({ instance: { ...instance, status: 'FAILED', serviceUrl } });
      }

      if (liveStatus === 'STOPPED') {
        await updateInstanceStatus(instanceId, 'STOPPED');
        return NextResponse.json({ instance: { ...instance, status: 'STOPPED' } });
      }

      if (liveStatus === 'RUNNING') {
        let readyAt = instance.cloudRunReadyAt;
        if (!readyAt) {
          readyAt = new Date().toISOString();
          await updateInstanceStatus(instanceId, 'PROVISIONING', {
            cloudRunReadyAt: readyAt,
            serviceUrl: serviceUrl || instance.serviceUrl,
          });
        }

        const elapsedMs = Date.now() - new Date(readyAt).getTime();

        if (elapsedMs >= IAP_PROPAGATION_DELAY_MS) {
          if (instance.status !== 'RUNNING' || (serviceUrl && serviceUrl !== instance.serviceUrl)) {
            await updateInstanceStatus(instanceId, 'RUNNING', {
              serviceUrl: serviceUrl || instance.serviceUrl,
            });
          }
          return NextResponse.json({
            instance: {
              ...instance,
              status: 'RUNNING',
              cloudRunReadyAt: readyAt,
              serviceUrl: serviceUrl || instance.serviceUrl,
            },
          });
        } else {
          return NextResponse.json({
            instance: {
              ...instance,
              status: 'PROVISIONING',
              cloudRunReadyAt: readyAt,
              serviceUrl: serviceUrl || instance.serviceUrl,
            },
          });
        }
      }
    }

    return NextResponse.json({ instance });
  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}

export async function DELETE(
  req: NextRequest,
  { params }: { params: { id: string } }
) {
  try {
    const instanceId = params.id;
    const instance = await getInstance(instanceId);

    if (!instance) {
      return NextResponse.json({ error: 'Instance not found.' }, { status: 404 });
    }

    await updateInstanceStatus(instanceId, 'STOPPING');
    await deleteDemoService(instanceId);
    await deleteInstanceRecord(instanceId);

    return NextResponse.json({ message: `Instance ${instanceId} torn down successfully.` });
  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}

