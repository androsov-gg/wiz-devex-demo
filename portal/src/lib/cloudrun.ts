import { ServicesClient, protos } from '@google-cloud/run';
import { Logging } from '@google-cloud/logging';
import { GoogleAuth } from 'google-auth-library';
import { DemoInstance, DemoTemplate, InstanceStatus } from './types';

let runClient: ServicesClient | null = null;
let loggingClient: Logging | null = null;

function getRunServicesClient(): ServicesClient {
  if (runClient) return runClient;

  runClient = new ServicesClient();
  return runClient;
}

function getLoggingClient(): Logging {
  if (loggingClient) return loggingClient;
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID;
  loggingClient = projectId ? new Logging({ projectId }) : new Logging();
  return loggingClient;
}

/**
 * Helper to dynamically resolve Google Cloud Project numeric ID if not in env.
 */
async function getResolvedProjectNumber(projectId: string): Promise<string> {
  if (process.env.PROJECT_NUMBER && process.env.PROJECT_NUMBER !== '1234567890') {
    return process.env.PROJECT_NUMBER;
  }
  try {
    const res = await fetch('http://metadata.google.internal/computeMetadata/v1/project/numeric-project-id', {
      headers: { 'Metadata-Flavor': 'Google' },
      signal: AbortSignal.timeout(1500),
    });
    if (res.ok) {
      const num = (await res.text()).trim();
      if (num) return num;
    }
  } catch {}
  return process.env.PROJECT_NUMBER || '';
}

/**
 * Deploys a dedicated Cloud Run v2 service for a demo instance with IAP and IAM pre-configured.
 */
export async function deployDemoService(
  instance: DemoInstance,
  template: DemoTemplate,
  imageUri: string
): Promise<{ serviceUrl?: string }> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';
  const projectNumber = await getResolvedProjectNumber(projectId);
  const runnerSa = process.env.DEMO_RUNNER_SA_EMAIL || `wiz-demo-runner-sa@${projectId}.iam.gserviceaccount.com`;
  const port = template.port || 8080;
  const serviceId = instance.id.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  const parent = `projects/${projectId}/locations/${region}`;

  const client = getRunServicesClient();

  const envVars = [
    { name: 'GOOGLE_CLOUD_PROJECT', value: projectId },
    { name: 'GOOGLE_CLOUD_LOCATION', value: region },
    { name: 'ANTHROPIC_VERTEX_PROJECT_ID', value: projectId },
    { name: 'ANTHROPIC_VERTEX_LOCATION', value: region },
    { name: 'CLOUD_ML_REGION', value: region },
    ...Object.entries(template.defaultEnv || {})
      .filter(([name]) => name.toUpperCase() !== 'PORT')
      .map(([name, value]) => ({ name, value })),
  ];

  // 1. Define service specification with manual scaling
  const servicePayload: protos.google.cloud.run.v2.IService = {
    ingress: 'INGRESS_TRAFFIC_ALL',
    scaling: {
      scalingMode: "MANUAL",
      manualInstanceCount: 1,
    },
    template: {
      serviceAccount: runnerSa,
      sessionAffinity: true,
      timeout: { seconds: 3600 },
      scaling: {
        minInstanceCount: 1,
        maxInstanceCount: 1,
      },
      containers: [
        {
          image: imageUri,
          ports: [{ containerPort: port }],
          resources: {
            limits: {
              cpu: template.cpu,
              memory: template.memory
            },
          },
          env: envVars,
        },
      ],
      labels: {
        app: 'wiz-demo-platform',
        'demo-id': serviceId,
        'template-id': template.id,
      },
    },
    labels: {
      app: 'wiz-demo-platform',
      'demo-id': serviceId,
      'template-id': template.id,
    },
  };

  try {
    console.log(`Creating Cloud Run v2 service ${serviceId} in ${parent}...`);
    const [operation] = await client.createService({
      parent,
      serviceId,
      service: servicePayload,
    });

    const [response] = await operation.promise();
    const serviceUrl = response.uri || `https://${serviceId}-${projectId.substring(0, 8)}.${region}.a.run.app`;

    // 2. Configure IAP, invoker IAM, and web access policy
    await ensureIapConfigured(serviceId, instance.ownerEmail);

    return { serviceUrl };
  } catch (err: any) {
    console.error(`Error deploying Cloud Run service ${serviceId}:`, err);
    throw new Error(`Cloud Run deployment failed: ${err?.message || err}`);
  }
}

/**
 * Ensures IAP is enabled and properly configured on a Cloud Run service (REST patch, IAM invoker, and IAP policy).
 */
export async function ensureIapConfigured(serviceId: string, ownerEmail?: string): Promise<void> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';
  const projectNumber = await getResolvedProjectNumber(projectId);
  const client = getRunServicesClient();

  const auth = new GoogleAuth({
    scopes: ['https://www.googleapis.com/auth/cloud-platform'],
  });
  const authClient = await auth.getClient();

  // 1. Enable IAP on the Cloud Run service via v2 REST API
  try {
    console.log(`Enabling IAP on Cloud Run service ${serviceId}...`);
    const patchUrl = `https://run.googleapis.com/v2/projects/${projectId}/locations/${region}/services/${serviceId}?updateMask=iapEnabled`;
    await authClient.request({
      url: patchUrl,
      method: 'PATCH',
      data: {
        iapEnabled: true,
      },
    });
    console.log(`Successfully enabled IAP on Cloud Run service ${serviceId}`);
  } catch (iapPatchErr: any) {
    console.warn(`Warning: Could not enable IAP on Cloud Run service ${serviceId}:`, iapPatchErr?.message || iapPatchErr);
  }

  // 2. Configure IAM: Grant roles/run.invoker to the IAP service agent
  try {
    const resource = `projects/${projectId}/locations/${region}/services/${serviceId}`;
    const [existingPolicy] = await client.getIamPolicy({ resource });
    const bindings = existingPolicy?.bindings || [];
    const invokerRole = 'roles/run.invoker';
    const iapServiceAgent = projectNumber
      ? `serviceAccount:service-${projectNumber}@gcp-sa-iap.iam.gserviceaccount.com`
      : '';

    if (iapServiceAgent) {
      let found = false;
      for (const b of bindings) {
        if (b.role === invokerRole) {
          if (!b.members?.includes(iapServiceAgent)) {
            b.members = [...(b.members || []), iapServiceAgent];
          }
          found = true;
          break;
        }
      }
      if (!found) {
        bindings.push({ role: invokerRole, members: [iapServiceAgent] });
      }

      await client.setIamPolicy({
        resource,
        policy: {
          ...existingPolicy,
          bindings,
        },
      });
    }
  } catch (iamErr: any) {
    console.warn(`Warning: Could not set IAM invoker policy for ${serviceId}:`, iamErr?.message || iamErr);
  }

  // 3. Configure IAP Web Policy: Grant roles/iap.httpsResourceAccessor to users
  const rawIapMembers = process.env.IAP_MEMBERS || '';
  const iapMembersList = rawIapMembers.split(',').map((m) => m.trim()).filter(Boolean);
  if (ownerEmail && !iapMembersList.includes(`user:${ownerEmail}`) && !iapMembersList.includes(ownerEmail)) {
    iapMembersList.push(`user:${ownerEmail}`);
  }

  if (iapMembersList.length > 0) {
    try {
      const iapTarget = projectNumber || projectId;
      const iapResource = `projects/${iapTarget}/iap_web/cloud_run-${region}/services/${serviceId}`;
      const url = `https://iap.googleapis.com/v1/${iapResource}:setIamPolicy`;

      await authClient.request({
        url,
        method: 'POST',
        data: {
          policy: {
            bindings: [
              {
                role: 'roles/iap.httpsResourceAccessor',
                members: iapMembersList,
              },
            ],
          },
        },
      });
      console.log(`Successfully configured IAP policy for ${serviceId} with members:`, iapMembersList);
    } catch (iapErr: any) {
      console.warn(`Could not set IAP web policy for ${serviceId}:`, iapErr.message);
    }
  }
}

/**
 * Checks the live status of a Cloud Run demo service.
 */
export async function getDemoServiceStatus(instanceId: string): Promise<{
  status: InstanceStatus;
  serviceUrl?: string;
  ready: boolean;
}> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';
  const serviceId = instanceId.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  const name = `projects/${projectId}/locations/${region}/services/${serviceId}`;

  const client = getRunServicesClient();
  if (!client) {
    return { status: 'RUNNING', serviceUrl: `https://${serviceId}-mock.a.run.app`, ready: true };
  }

  try {
    const [service] = await client.getService({ name });
    const serviceUrl = service.uri || undefined;

    const terminalCondition = service.terminalCondition;
    const conditions = service.conditions || [];
    const readyCondition = conditions.find(c => c.type === 'Ready') || terminalCondition;

    const isSucceeded =
      terminalCondition?.state === 'CONDITION_SUCCEEDED' ||
      terminalCondition?.state === 4 ||
      readyCondition?.state === 'CONDITION_SUCCEEDED' ||
      readyCondition?.state === 4 ||
      (service.reconciling === false && !!serviceUrl && terminalCondition?.state !== 'CONDITION_FAILED' && terminalCondition?.state !== 3);

    const isFailed =
      terminalCondition?.state === 'CONDITION_FAILED' ||
      terminalCondition?.state === 3 ||
      readyCondition?.state === 'CONDITION_FAILED' ||
      readyCondition?.state === 3;

    if (isSucceeded) {
      return { status: 'RUNNING', serviceUrl, ready: true };
    }
    if (isFailed) {
      return { status: 'FAILED', serviceUrl, ready: false };
    }
    return { status: 'PROVISIONING', serviceUrl, ready: false };
  } catch (err: any) {
    // 404 / NOT_FOUND means service does not exist
    if (err?.code === 5 || err?.status === 404 || err?.message?.includes('NOT_FOUND')) {
      return { status: 'STOPPED', ready: false };
    }
    console.warn(`Status check error for ${serviceId}:`, err.message);
    return { status: 'PROVISIONING', ready: false };
  }
}

/**
 * Deletes a Cloud Run demo service upon teardown.
 */
export async function deleteDemoService(instanceId: string): Promise<void> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';
  const serviceId = instanceId.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  const name = `projects/${projectId}/locations/${region}/services/${serviceId}`;

  const client = getRunServicesClient();

  try {
    const [operation] = await client.deleteService({ name });
    await operation.promise();
    console.log(`Successfully deleted Cloud Run service ${serviceId}`);
  } catch (err: any) {
    if (err?.code === 5 || err?.status === 404 || err?.message?.includes('NOT_FOUND')) {
      return; // Already deleted
    }
    console.warn(`Could not delete Cloud Run service ${serviceId}:`, err.message);
  }
}

/**
 * Triggers a restart of the Cloud Run demo service by deploying a new revision with IAP preserved.
 */
export async function restartDemoService(instanceId: string, ownerEmail?: string): Promise<void> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';
  const serviceId = instanceId.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  const name = `projects/${projectId}/locations/${region}/services/${serviceId}`;

  const client = getRunServicesClient();

  try {
    const [service] = await client.getService({ name });
    if (!service.template) service.template = {};
    if (!service.template.annotations) service.template.annotations = {};

    // Touch restart annotation to trigger fresh rollout (avoiding system annotation namespaces)
    service.template.annotations['wiz.dev/restartedAt'] = new Date().toISOString();

    (service as any).iapEnabled = true;

    // Use updateMask to only update template.annotations so service-level configs like IAP are not wiped out
    const [operation] = await client.updateService({
      service,
      updateMask: {
        paths: ['template.annotations'],
      },
    });
    await operation.promise();
    console.log(`Successfully restarted Cloud Run service ${serviceId}`);

    // Re-verify and ensure IAP is enabled and configured after restart
    await ensureIapConfigured(serviceId, ownerEmail);
  } catch (err: any) {
    console.error(`Failed to restart Cloud Run service ${serviceId}:`, err);
    throw err;
  }
}

/**
 * Fetches container logs from Google Cloud Logging.
 */
export async function getDemoServiceLogs(instanceId: string, lines = 100): Promise<string> {
  const serviceId = instanceId.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  const logging = getLoggingClient();

  try {
    const filter = `resource.type="cloud_run_revision" AND resource.labels.service_name="${serviceId}"`;
    const [entries] = await logging.getEntries({
      filter,
      pageSize: lines,
      orderBy: 'timestamp desc',
    });

    if (!entries || entries.length === 0) {
      return 'No logs found yet for this Cloud Run service.';
    }

    const logLines = entries
      .reverse()
      .map((entry) => {
        let textPayload = '';
        if (typeof entry.data === 'string') {
          textPayload = entry.data;
        } else if (entry.data && typeof entry.data === 'object') {
          textPayload = (entry.data as any).message || JSON.stringify(entry.data);
        } else if (entry.metadata?.textPayload) {
          textPayload = entry.metadata.textPayload;
        } else if (entry.metadata?.jsonPayload) {
          textPayload = typeof entry.metadata.jsonPayload === 'string' ? entry.metadata.jsonPayload : JSON.stringify(entry.metadata.jsonPayload);
        } else if (entry.data) {
          textPayload = String(entry.data);
        }
        const timestamp = entry.metadata?.timestamp ? new Date(entry.metadata.timestamp as any).toISOString() : '';
        return timestamp ? `[${timestamp}] ${textPayload}` : textPayload;
      })
      .filter(Boolean);

    return logLines.join('\n') || 'No log output recorded yet.';
  } catch (err: any) {
    return `Error fetching logs: ${err?.message || err}`;
  }
}
