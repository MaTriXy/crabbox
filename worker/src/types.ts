export interface Env {
  FLEET: DurableObjectNamespace;
  HETZNER_TOKEN: string;
  AWS_ACCESS_KEY_ID?: string;
  AWS_SECRET_ACCESS_KEY?: string;
  AWS_SESSION_TOKEN?: string;
  CRABBOX_AWS_REGION?: string;
  CRABBOX_AWS_AMI?: string;
  CRABBOX_AWS_SECURITY_GROUP_ID?: string;
  CRABBOX_AWS_SUBNET_ID?: string;
  CRABBOX_AWS_INSTANCE_PROFILE?: string;
  CRABBOX_AWS_ROOT_GB?: string;
  CRABBOX_SHARED_TOKEN?: string;
}

export interface LeaseRequest {
  provider?: Provider;
  profile?: string;
  class?: string;
  serverType?: string;
  location?: string;
  image?: string;
  awsRegion?: string;
  awsAMI?: string;
  awsSGID?: string;
  awsSubnetID?: string;
  awsProfile?: string;
  awsRootGB?: number;
  sshUser?: string;
  sshPort?: string;
  providerKey?: string;
  workRoot?: string;
  ttlSeconds?: number;
  keep?: boolean;
  sshPublicKey?: string;
}

export type Provider = "hetzner" | "aws";

export interface LeaseRecord {
  id: string;
  provider: Provider;
  cloudID: string;
  region?: string;
  owner: string;
  profile: string;
  class: string;
  serverType: string;
  serverID: number;
  serverName: string;
  host: string;
  sshUser: string;
  sshPort: string;
  workRoot: string;
  keep: boolean;
  state: "active" | "released" | "expired" | "failed";
  createdAt: string;
  updatedAt: string;
  expiresAt: string;
}

export interface HetznerServer {
  id: number;
  name: string;
  status: string;
  labels: Record<string, string>;
  public_net: {
    ipv4: {
      ip: string;
    };
  };
  server_type: {
    name: string;
  };
}

export interface HetznerSSHKey {
  id: number;
  name: string;
  fingerprint: string;
  public_key: string;
}

export interface MachineView {
  id: string;
  provider: Provider;
  cloudID: string;
  name: string;
  status: string;
  serverType: string;
  host: string;
  labels: Record<string, string>;
}

export interface ProviderMachine {
  provider: Provider;
  id: number;
  cloudID: string;
  name: string;
  status: string;
  serverType: string;
  host: string;
  labels: Record<string, string>;
}
