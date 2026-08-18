import { CloudBuildClient } from '@google-cloud/cloudbuild';
import { ArtifactRegistryClient } from '@google-cloud/artifact-registry';
import { BuildRecord } from './types';
import { saveBuildRecord } from './firestore';

let cbClient: CloudBuildClient | null = null;
let arClient: ArtifactRegistryClient | null = null;

function getCloudBuildClient(): CloudBuildClient | null {
  if (cbClient) return cbClient;
  try {
    cbClient = new CloudBuildClient();
    return cbClient;
  } catch (err) {
    console.warn('⚠️ CloudBuildClient init failed:', err);
    return null;
  }
}

function getArtifactRegistryClient(): ArtifactRegistryClient | null {
  if (arClient) return arClient;
  try {
    arClient = new ArtifactRegistryClient();
    return arClient;
  } catch (err) {
    console.warn('⚠️ ArtifactRegistryClient init failed:', err);
    return null;
  }
}

/**
 * Checks if a specific container image tag exists in Artifact Registry.
 */
export async function checkImageExists(imageTag: string): Promise<boolean> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';

  // Tag format: ${region}-docker.pkg.dev/${projectId}/wiz-devex-demo-images/${imageName}:${tag}
  try {
    const parts = imageTag.split('/');
    const repoAndImage = parts[parts.length - 1]; // e.g. "wiz-code-golden-demo:1.0.0"
    const [imageName, tag = 'latest'] = repoAndImage.split(':');

    const client = getArtifactRegistryClient();
    if (!client) {
      // In local dev without credentials, assume image exists if simulation mode
      return process.env.NODE_ENV === 'development';
    }

    const tagResourcePath = `projects/${projectId}/locations/${region}/repositories/wiz-devex-demo-images/packages/${imageName}/tags/${tag}`;
    const [tagData] = await client.getTag({ name: tagResourcePath });
    return !!tagData?.version;
  } catch (err: any) {
    // 404 or NOT_FOUND means tag does not exist
    if (err?.code === 5 || err?.message?.includes('NOT_FOUND') || err?.status === 404) {
      return false;
    }
    console.warn('Artifact Registry check error (assuming missing):', err?.message || err);
    return false;
  }
}

/**
 * Triggers a Cloud Build job to pull configuration from GCS and build/push the container image.
 */
export async function triggerCloudBuild(
  configName: string,
  version: string,
  targetImageUri: string
): Promise<string> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const configBucket =
    process.env.CONFIG_BUCKET_NAME ||
    process.env.CONFIG_BUCKET ||
    `${projectId}-wiz-devex-demo-configs`;
  const client = getCloudBuildClient();

  const buildId = `build-${configName}-${version}-${Date.now()}`;

  if (!client) {
    console.log(`[DEV MODE] Simulated Cloud Build trigger for ${configName}:${version} -> ${targetImageUri}`);
    await saveBuildRecord({
      buildId,
      templateId: configName,
      version,
      status: 'SUCCESS',
      startTime: new Date().toISOString(),
      finishTime: new Date().toISOString(),
      logUrl: `https://console.cloud.google.com/cloud-build/builds?project=${projectId}`,
    });
    return buildId;
  }

  try {
    const builderSa = `projects/${projectId}/serviceAccounts/cloudbuild-builder-sa@${projectId}.iam.gserviceaccount.com`;

    const [operation] = await client.createBuild({
      projectId,
      build: {
        serviceAccount: builderSa,
        steps: [
          {
            name: 'gcr.io/google.com/cloudsdktool/cloud-sdk:slim',
            entrypoint: 'sh',
            args: [
              '-c',
              `gcloud storage rsync --recursive "gs://${configBucket}/${configName}/${version}" . || gsutil -m rsync -r "gs://${configBucket}/${configName}/${version}" .`,
            ],
          },
          {
            name: 'gcr.io/cloud-builders/docker',
            env: ['DOCKER_BUILDKIT=1'],
            args: ['build', '-t', targetImageUri, '.'],
          },
        ],
        images: [targetImageUri],
        options: {
          logging: 'CLOUD_LOGGING_ONLY',
        },
      },
    });

    const metadata = operation.metadata as any;
    const realBuildId = metadata?.build?.id || operation.name || buildId;
    const logUrl =
      metadata?.build?.logUrl ||
      `https://console.cloud.google.com/cloud-build/builds;region=global/${realBuildId}?project=${projectId}`;

    await saveBuildRecord({
      buildId: realBuildId,
      templateId: configName,
      version,
      status: 'WORKING',
      startTime: new Date().toISOString(),
      logUrl,
    });

    return realBuildId;
  } catch (err: any) {
    console.error('Failed to trigger Cloud Build:', err);
    throw new Error(`Cloud Build failed to trigger: ${err.message}`);
  }
}

/**
 * Queries current Cloud Build status.
 */
export async function getCloudBuildStatus(buildId: string): Promise<{ status: BuildRecord['status']; logUrl?: string }> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const client = getCloudBuildClient();

  if (!client || buildId.startsWith('build-')) {
    return { status: 'SUCCESS' };
  }

  try {
    let build: any;
    try {
      const [res] = await client.getBuild({ projectId, id: buildId });
      build = res;
    } catch (e1) {
      const [res] = await client.getBuild({ name: `projects/${projectId}/locations/global/builds/${buildId}` });
      build = res;
    }

    const statusMap: Record<string, BuildRecord['status']> = {
      QUEUED: 'QUEUED',
      WORKING: 'WORKING',
      SUCCESS: 'SUCCESS',
      FAILURE: 'FAILURE',
      CANCELLED: 'CANCELLED',
      TIMEOUT: 'FAILURE',
    };

    const status = statusMap[build?.status || ''] || 'WORKING';
    const logUrl = build?.logUrl || `https://console.cloud.google.com/cloud-build/builds;region=global/${buildId}?project=${projectId}`;

    return { status, logUrl };
  } catch (err: any) {
    console.warn(`Failed to fetch Cloud Build status for ${buildId}:`, err.message);
    return { status: 'WORKING' };
  }
}
