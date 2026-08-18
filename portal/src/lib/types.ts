export type DemoCategory = 'Application Security' | 'AI & DevEx' | 'Cloud Security' | 'Container Security' | 'IaC Security';

export interface DemoVersion {
  version: string;
  description: string;
}

export interface DemoTemplate {
  id: string;
  name: string;
  title?: string;
  description: string;
  category: DemoCategory;
  module: string;
  icon: string;
  containerImage: string;
  buildSourcePath: string;
  port: number;
  cpu: string;
  memory: string;
  defaultEnv?: Record<string, string>;
  docsUrl?: string;
  versions: DemoVersion[];
  selectedVersion?: string;
}

export type InstanceStatus =
  | 'BUILDING_IMAGE'
  | 'PROVISIONING'
  | 'RESTARTING'
  | 'RUNNING'
  | 'STOPPING'
  | 'STOPPED'
  | 'FAILED';

export interface DemoInstance {
  id: string;
  templateId: string;
  templateName: string;
  version?: string;
  ownerEmail: string;
  ownerName: string;
  status: InstanceStatus;
  createdAt: string;
  serviceName?: string;
  serviceUrl?: string;
  region?: string;
  port: number;
  buildId?: string;
  errorMessage?: string;
  cloudRunReadyAt?: string;
}

export interface BuildRecord {
  buildId: string;
  templateId: string;
  version?: string;
  status: 'QUEUED' | 'WORKING' | 'SUCCESS' | 'FAILURE' | 'CANCELLED';
  startTime: string;
  finishTime?: string;
  logUrl?: string;
}

export interface UserSession {
  email: string;
  name: string;
  initials: string;
  domain: string;
}
