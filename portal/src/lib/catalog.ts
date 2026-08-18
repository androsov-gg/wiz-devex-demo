import { Storage } from '@google-cloud/storage';
import yaml from 'js-yaml';
import fs from 'fs';
import path from 'path';
import { DemoTemplate, DemoVersion, DemoCategory } from './types';

let storageClient: Storage | null = null;

function getStorageClient(): Storage | null {
  if (storageClient) return storageClient;
  try {
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID;
    storageClient = new Storage({
      projectId: projectId || undefined,
    });
    return storageClient;
  } catch (err) {
    console.warn('⚠️ Cloud Storage client initialization warning:', err);
    return null;
  }
}

// In-memory cache for catalog templates with 30s TTL
let cachedCatalog: DemoTemplate[] | null = null;
let cacheExpiry = 0;

/**
 * Compares two semantic version strings (e.g. '1.2.0', '1.0.0', 'v2.1.0', '1.0.0-rc.1').
 * Returns > 0 if a > b, < 0 if a < b, 0 if equal.
 */
export function compareSemver(a: string, b: string): number {
  const cleanA = (a || '').replace(/^v/i, '').trim();
  const cleanB = (b || '').replace(/^v/i, '').trim();

  const [verA, preA] = cleanA.split('-');
  const [verB, preB] = cleanB.split('-');

  const partsA = (verA || '0').split('.').map((p) => parseInt(p, 10) || 0);
  const partsB = (verB || '0').split('.').map((p) => parseInt(p, 10) || 0);

  const maxLen = Math.max(partsA.length, partsB.length, 3);
  for (let i = 0; i < maxLen; i++) {
    const numA = partsA[i] ?? 0;
    const numB = partsB[i] ?? 0;
    if (numA !== numB) {
      return numA - numB;
    }
  }

  // Pre-release versions have lower precedence than normal version
  if (!preA && preB) return 1;
  if (preA && !preB) return -1;
  if (preA && preB) return preA.localeCompare(preB);

  return 0;
}

function parseVersions(versionsRaw: any): DemoVersion[] {
  if (!versionsRaw) {
    return [{ version: '1.0.0', description: '' }];
  }

  let list: DemoVersion[] = [];

  if (Array.isArray(versionsRaw)) {
    list = versionsRaw.map((v) => ({
      version: `${v.version || '1.0.0'}`,
      description: v.description || '',
    }));
  } else if (typeof versionsRaw === 'object') {
    list = Object.entries(versionsRaw).map(([verKey, verVal]: [string, any]) => {
      if (typeof verVal === 'string') {
        return { version: verKey, description: verVal };
      }
      return {
        version: verVal?.version || verKey,
        description: verVal?.description || '',
      };
    });
  } else {
    list = [{ version: '1.0.0', description: '' }];
  }

  // Sort descending by semver: latest version appears first
  return list.sort((a, b) => compareSemver(b.version, a.version));
}

function parseCatalogYaml(yamlContent: string): DemoTemplate[] {
  const parsed = yaml.load(yamlContent) as Record<string, any>;
  if (!parsed || typeof parsed !== 'object') {
    return [];
  }

  const templates: DemoTemplate[] = [];

  for (const [key, val] of Object.entries(parsed)) {
    if (!val || typeof val !== 'object') continue;

    const versions = parseVersions(val.versions);
    const primaryVersion = versions[0]?.version || '1.0.0';
    const primaryDescription = val.description || versions[0]?.description || '';

    templates.push({
      id: key,
      name: val.title || val.name || key,
      title: val.title || val.name || key,
      description: primaryDescription,
      category: (val.category || 'AI & DevEx') as DemoCategory,
      module: val.module || '',
      icon: val.icon || 'Code2',
      containerImage: `${key}:${primaryVersion}`,
      buildSourcePath: `configurations/${key}/${primaryVersion}`,
      port: typeof val.port === 'number' ? val.port : 8080,
      cpu: `${val.cpu || '2'}`,
      memory: `${val.memory || '4Gi'}`,
      defaultEnv: val.defaultEnv && typeof val.defaultEnv === 'object' ? val.defaultEnv : {},
      docsUrl: val.docsUrl || undefined,
      versions,
      selectedVersion: primaryVersion,
    });
  }

  return templates;
}

export async function getDemoCatalog(forceRefresh = false): Promise<DemoTemplate[]> {
  const now = Date.now();
  if (!forceRefresh && cachedCatalog && now < cacheExpiry) {
    return cachedCatalog;
  }

  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  const bucketName =
    process.env.CONFIG_BUCKET_NAME ||
    process.env.CONFIG_BUCKET ||
    `${projectId}-wiz-devex-demo-configs`;

  let yamlContent: string | null = null;

  // 1. Try pulling index.yaml from Cloud Storage bucket
  const storage = getStorageClient();
  if (storage) {
    try {
      const bucket = storage.bucket(bucketName);
      const file = bucket.file('index.yaml');
      const [exists] = await file.exists();
      if (exists) {
        const [contents] = await file.download();
        yamlContent = contents.toString('utf-8');
      }
    } catch (err: any) {
      console.warn(`Could not read index.yaml from gs://${bucketName}:`, err?.message || err);
    }
  }

  // 2. Fallback to local configurations/index.yaml for local development
  if (!yamlContent) {
    const candidatePaths = [
      path.join(process.cwd(), 'configurations', 'index.yaml'),
      path.join(process.cwd(), '..', 'configurations', 'index.yaml'),
      path.resolve(__dirname, '../../../../configurations/index.yaml'),
    ];

    for (const p of candidatePaths) {
      if (fs.existsSync(p)) {
        try {
          yamlContent = fs.readFileSync(p, 'utf-8');
          break;
        } catch (readErr) {
          // continue
        }
      }
    }
  }

  if (yamlContent) {
    try {
      const templates = parseCatalogYaml(yamlContent);
      cachedCatalog = templates;
      cacheExpiry = now + 30000; // 30 seconds TTL
      return templates;
    } catch (parseErr) {
      console.error('Failed to parse catalog YAML:', parseErr);
    }
  }

  return cachedCatalog || [];
}

export async function getDemoTemplate(id: string, version?: string): Promise<DemoTemplate | undefined> {
  const catalog = await getDemoCatalog();
  const template = catalog.find((t) => t.id === id);
  if (!template) return undefined;

  if (version) {
    const matchedVer = template.versions.find((v) => v.version === version);
    const verName = matchedVer ? matchedVer.version : version;
    const verDesc = matchedVer?.description || template.description;

    return {
      ...template,
      selectedVersion: verName,
      description: verDesc,
      containerImage: `${template.id}:${verName}`,
      buildSourcePath: `configurations/${template.id}/${verName}`,
    };
  }

  return template;
}

export function getFullImageUri(imageName: string): string {
  const region = process.env.GOOGLE_CLOUD_LOCATION || process.env.REGION || 'europe-west1';
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID || 'wiz-code-demo';
  return `${region}-docker.pkg.dev/${projectId}/wiz-devex-demo-images/${imageName}`;
}
