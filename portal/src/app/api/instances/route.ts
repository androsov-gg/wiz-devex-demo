import { NextRequest, NextResponse } from 'next/server';
import { getDemoTemplate, getFullImageUri } from '@/lib/catalog';
import { listInstances, saveInstance, updateInstanceStatus } from '@/lib/firestore';
import { checkImageExists, triggerCloudBuild, getCloudBuildStatus } from '@/lib/cloudbuild';
import { deployDemoService, getDemoServiceStatus } from '@/lib/cloudrun';
import { DemoInstance } from '@/lib/types';

export const dynamic = 'force-dynamic';

const IAP_PROPAGATION_DELAY_MS = 90 * 1000; // 90 second delay for IAP propagation

export async function GET(req: NextRequest) {
  try {
    const instances = await listInstances();

    // Reconcile state with live Cloud Run service state and Cloud Build status
    const updatedInstances = await Promise.all(
      instances.map(async (inst) => {
        // 1. In-flight Cloud Build / Image status reconciliation
        if (inst.status === 'BUILDING_IMAGE') {
          try {
            const template = await getDemoTemplate(inst.templateId, inst.version);
            if (template) {
              const targetImageUri = getFullImageUri(template.containerImage);
              const imageExistsInAr = await checkImageExists(targetImageUri);

              let isSuccess = imageExistsInAr;
              if (!isSuccess && inst.buildId) {
                const { status: buildStatus } = await getCloudBuildStatus(inst.buildId);
                isSuccess = buildStatus === 'SUCCESS';
                if (buildStatus === 'FAILURE' || buildStatus === 'CANCELLED') {
                  const errorMsg = 'Cloud Build failed to compile container image.';
                  await updateInstanceStatus(inst.id, 'FAILED', { errorMessage: errorMsg });
                  return { ...inst, status: 'FAILED' as const, errorMessage: errorMsg };
                }
              }

              if (isSuccess) {
                await updateInstanceStatus(inst.id, 'PROVISIONING');
                const { serviceUrl } = await deployDemoService(inst, template, targetImageUri);
                const readyTimestamp = new Date().toISOString();
                await updateInstanceStatus(inst.id, 'PROVISIONING', {
                  cloudRunReadyAt: readyTimestamp,
                  serviceUrl,
                });
                return {
                  ...inst,
                  status: 'PROVISIONING' as const,
                  cloudRunReadyAt: readyTimestamp,
                  serviceUrl,
                };
              }
            }
          } catch (cbErr: any) {
            console.warn(`Error reconciling build/image for instance ${inst.id}:`, cbErr.message);
          }
        }

        // 2. Live Cloud Run service reconciliation
        if (inst.status === 'RUNNING' || inst.status === 'PROVISIONING' || inst.status === 'RESTARTING') {
          const { status: liveStatus, serviceUrl } = await getDemoServiceStatus(inst.id);

          if (liveStatus === 'FAILED') {
            await updateInstanceStatus(inst.id, 'FAILED', { serviceUrl });
            return { ...inst, status: 'FAILED' as const, serviceUrl: serviceUrl || inst.serviceUrl };
          }

          if (liveStatus === 'STOPPED') {
            await updateInstanceStatus(inst.id, 'STOPPED');
            return { ...inst, status: 'STOPPED' as const };
          }

          if (liveStatus === 'RUNNING') {
            let readyAt = inst.cloudRunReadyAt;
            if (!readyAt) {
              readyAt = new Date().toISOString();
              await updateInstanceStatus(inst.id, 'PROVISIONING', {
                cloudRunReadyAt: readyAt,
                serviceUrl: serviceUrl || inst.serviceUrl,
              });
            }

            const elapsedMs = Date.now() - new Date(readyAt).getTime();

            if (elapsedMs >= IAP_PROPAGATION_DELAY_MS) {
              if (inst.status !== 'RUNNING' || (serviceUrl && serviceUrl !== inst.serviceUrl)) {
                await updateInstanceStatus(inst.id, 'RUNNING', {
                  serviceUrl: serviceUrl || inst.serviceUrl,
                });
              }
              return {
                ...inst,
                status: 'RUNNING' as const,
                cloudRunReadyAt: readyAt,
                serviceUrl: serviceUrl || inst.serviceUrl,
              };
            } else {
              // Within the 90s IAP propagation delay window
              return {
                ...inst,
                status: 'PROVISIONING' as const,
                cloudRunReadyAt: readyAt,
                serviceUrl: serviceUrl || inst.serviceUrl,
              };
            }
          }
        }
        return inst;
      })
    );

    return NextResponse.json({ instances: updatedInstances });
  } catch (err: any) {
    console.error('Error fetching instances:', err);
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}

export async function POST(req: NextRequest) {
  try {
    const body = await req.json();
    const { templateId, version, customName } = body;

    const template = await getDemoTemplate(templateId, version);
    if (!template) {
      return NextResponse.json({ error: `Template '${templateId}' not found.` }, { status: 404 });
    }

    const selectedVersion = template.selectedVersion || version || '1.0.0';

    // Extract authenticated user from IAP header, with fallback
    const rawIapEmail = req.headers.get('x-goog-authenticated-user-email') || '';
    const ownerEmail = rawIapEmail.replace('accounts.google.com:', '') || 'ajkearns@altostrat.com';
    const ownerName = ownerEmail.split('@')[0];

    const randomSuffix = Math.random().toString(36).substring(2, 7);
    const instanceId = customName
      ? `${customName.toLowerCase().replace(/[^a-z0-9-]/g, '-')}-${randomSuffix}`
      : `${templateId}-${randomSuffix}`;

    const now = new Date();
    const targetImageUri = getFullImageUri(template.containerImage);

    // 1. Check if container image already exists in Artifact Registry
    const imageExists = await checkImageExists(targetImageUri);

    if (!imageExists) {
      console.log(`Image ${targetImageUri} does not exist in Artifact Registry. Triggering Cloud Build...`);

      let buildId: string;
      try {
        buildId = await triggerCloudBuild(template.id, selectedVersion, targetImageUri);
      } catch (buildErr: any) {
        return NextResponse.json(
          { error: `Cloud Build trigger failed: ${buildErr.message}` },
          { status: 500 }
        );
      }

      const instance: DemoInstance = {
        id: instanceId,
        templateId: template.id,
        templateName: template.name,
        version: selectedVersion,
        ownerEmail,
        ownerName,
        status: 'BUILDING_IMAGE',
        createdAt: now.toISOString(),
        port: template.port,
        buildId,
      };

      await saveInstance(instance);
      return NextResponse.json({ instance, status: 'BUILDING_IMAGE', buildId });
    }

    // 2. Image exists, deploy to Cloud Run directly
    const instance: DemoInstance = {
      id: instanceId,
      templateId: template.id,
      templateName: template.name,
      version: selectedVersion,
      ownerEmail,
      ownerName,
      status: 'PROVISIONING',
      createdAt: now.toISOString(),
      port: template.port,
    };

    await saveInstance(instance);

    // Asynchronously trigger Cloud Run deployment without blocking HTTP response
    deployDemoService(instance, template, targetImageUri)
      .then(async ({ serviceUrl }) => {
        const readyTimestamp = new Date().toISOString();
        await updateInstanceStatus(instance.id, 'PROVISIONING', {
          cloudRunReadyAt: readyTimestamp,
          serviceUrl,
        });
      })
      .catch(async (deployErr: any) => {
        console.error(`Async deployment error for instance ${instance.id}:`, deployErr);
        await updateInstanceStatus(instance.id, 'FAILED', { errorMessage: deployErr?.message || String(deployErr) });
      });

    return NextResponse.json({ instance, status: 'PROVISIONING' });
  } catch (err: any) {
    console.error('Launch demo error:', err);
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}

