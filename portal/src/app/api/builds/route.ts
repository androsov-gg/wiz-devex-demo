import { NextRequest, NextResponse } from 'next/server';
import { getCloudBuildStatus, checkImageExists } from '@/lib/cloudbuild';
import { getDemoTemplate, getFullImageUri } from '@/lib/catalog';
import { listInstances, updateInstanceStatus } from '@/lib/firestore';
import { deployDemoService } from '@/lib/cloudrun';

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  try {
    const buildId = req.nextUrl.searchParams.get('buildId');
    const instanceId = req.nextUrl.searchParams.get('instanceId');

    if (!buildId) {
      return NextResponse.json({ error: 'buildId query parameter is required.' }, { status: 400 });
    }

    const { status: cbStatus, logUrl } = await getCloudBuildStatus(buildId);

    // Look up matching instance in Firestore
    const instances = await listInstances();
    const inst = instances.find((i) => (instanceId && i.id === instanceId) || i.buildId === buildId);

    if (inst) {
      const template = await getDemoTemplate(inst.templateId, inst.version);
      if (template) {
        const targetImageUri = getFullImageUri(template.containerImage);
        const imageExistsInAr = await checkImageExists(targetImageUri);

        if (imageExistsInAr || cbStatus === 'SUCCESS') {
          if (inst.status === 'BUILDING_IMAGE') {
            await updateInstanceStatus(inst.id, 'PROVISIONING');
            const { serviceUrl } = await deployDemoService(inst, template, targetImageUri);
            const readyTimestamp = new Date().toISOString();
            await updateInstanceStatus(inst.id, 'PROVISIONING', {
              cloudRunReadyAt: readyTimestamp,
              serviceUrl,
            });
          }
          return NextResponse.json({ status: 'SUCCESS', logUrl });
        } else if (cbStatus === 'FAILURE' || cbStatus === 'CANCELLED') {
          await updateInstanceStatus(inst.id, 'FAILED', {
            errorMessage: 'Cloud Build failed to compile the container image.',
          });
          return NextResponse.json({ status: 'FAILURE', logUrl });
        }
      }
    }

    return NextResponse.json({ status: cbStatus, logUrl });
  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}
