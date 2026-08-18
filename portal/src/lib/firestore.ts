import { Firestore } from '@google-cloud/firestore';
import { DemoInstance, BuildRecord, InstanceStatus } from './types';

let db: Firestore | null = null;
const memoryStore: {
  instances: Map<string, DemoInstance>;
  builds: Map<string, BuildRecord>;
} = {
  instances: new Map(),
  builds: new Map(),
};

function getDb(): Firestore | null {
  if (db) return db;
  try {
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || process.env.PROJECT_ID;
    db = new Firestore({
      projectId: projectId || undefined,
    });
    return db;
  } catch (err) {
    console.warn('⚠️ Firestore initialization failed, using in-memory store:', err);
    return null;
  }
}

export async function saveInstance(instance: DemoInstance): Promise<void> {
  const firestore = getDb();
  if (firestore) {
    try {
      await firestore.collection('instances').doc(instance.id).set(instance, { merge: true });
      return;
    } catch (err) {
      console.warn('Firestore write failed, falling back to memory store:', err);
    }
  }
  memoryStore.instances.set(instance.id, instance);
}

export async function getInstance(id: string): Promise<DemoInstance | null> {
  const firestore = getDb();
  if (firestore) {
    try {
      const doc = await firestore.collection('instances').doc(id).get();
      if (doc.exists) {
        return doc.data() as DemoInstance;
      }
    } catch (err) {
      console.warn('Firestore read failed, checking memory store:', err);
    }
  }
  return memoryStore.instances.get(id) || null;
}

export async function listInstances(): Promise<DemoInstance[]> {
  const firestore = getDb();
  if (firestore) {
    try {
      const snapshot = await firestore.collection('instances').orderBy('createdAt', 'desc').get();
      if (!snapshot.empty) {
        return snapshot.docs.map((doc) => doc.data() as DemoInstance);
      }
    } catch (err) {
      console.warn('Firestore list failed, reading from memory store:', err);
    }
  }
  return Array.from(memoryStore.instances.values()).sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  );
}

export async function updateInstanceStatus(
  id: string,
  status: InstanceStatus,
  extra?: Partial<DemoInstance>
): Promise<void> {
  const firestore = getDb();
  const updatePayload = { status, ...(extra || {}) };
  if (firestore) {
    try {
      await firestore.collection('instances').doc(id).set(updatePayload, { merge: true });
      return;
    } catch (err) {
      console.warn('Firestore status update failed:', err);
    }
  }
  const existing = memoryStore.instances.get(id);
  if (existing) {
    memoryStore.instances.set(id, { ...existing, ...updatePayload });
  }
}

export async function deleteInstanceRecord(id: string): Promise<void> {
  const firestore = getDb();
  if (firestore) {
    try {
      await firestore.collection('instances').doc(id).delete();
    } catch (err) {
      console.warn('Firestore delete failed:', err);
    }
  }
  memoryStore.instances.delete(id);
}

export async function saveBuildRecord(build: BuildRecord): Promise<void> {
  const firestore = getDb();
  if (firestore) {
    try {
      await firestore.collection('builds').doc(build.buildId).set(build, { merge: true });
      return;
    } catch (err) {
      console.warn('Firestore build record failed:', err);
    }
  }
  memoryStore.builds.set(build.buildId, build);
}

export async function getBuildRecord(buildId: string): Promise<BuildRecord | null> {
  const firestore = getDb();
  if (firestore) {
    try {
      const doc = await firestore.collection('builds').doc(buildId).get();
      if (doc.exists) {
        return doc.data() as BuildRecord;
      }
    } catch (err) {
      console.warn('Firestore build read failed:', err);
    }
  }
  return memoryStore.builds.get(buildId) || null;
}
