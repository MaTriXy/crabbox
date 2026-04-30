import { EC2SpotClient } from "./aws";
import { leaseConfig } from "./config";
import { HetznerClient } from "./hetzner";
import { errorMessage, json, pathParts, readJson, requestOwner } from "./http";
import type { Env, LeaseRecord, LeaseRequest, Provider, ProviderMachine } from "./types";

const fleetID = "default";

export class FleetDurableObject implements DurableObject {
  constructor(
    private readonly state: DurableObjectState,
    private readonly env: Env,
  ) {}

  async fetch(request: Request): Promise<Response> {
    try {
      const parts = pathParts(request);
      const method = request.method.toUpperCase();
      if (method === "GET" && parts.join("/") === "v1/health") {
        return json({ ok: true, fleet: fleetID });
      }
      if (method === "GET" && parts.join("/") === "v1/pool") {
        return await this.pool(request);
      }
      if (method === "GET" && parts.join("/") === "v1/leases") {
        return await this.listLeases();
      }
      if (method === "POST" && parts.join("/") === "v1/leases") {
        return await this.createLease(request);
      }
      if (parts[0] === "v1" && parts[1] === "leases" && parts[2]) {
        return await this.leaseRoute(request, parts[2], parts[3]);
      }
      return json({ error: "not_found" }, { status: 404 });
    } catch (error) {
      return json({ error: errorMessage(error) }, { status: 500 });
    }
  }

  async alarm(): Promise<void> {
    await this.expireLeases();
    await this.scheduleAlarm();
  }

  private async createLease(request: Request): Promise<Response> {
    const owner = requestOwner(request);
    const input = await readJson<LeaseRequest>(request);
    const config = leaseConfig(input);
    const leaseID = newLeaseID();
    const provider = this.provider(config.provider, config.awsRegion);
    const { server, serverType } = await provider.createServerWithFallback(config, leaseID, owner);
    const now = new Date();
    const record: LeaseRecord = {
      id: leaseID,
      provider: config.provider,
      cloudID: server.cloudID,
      owner,
      profile: config.profile,
      class: config.class,
      serverType,
      serverID: server.id,
      serverName: server.name,
      host: server.host,
      sshUser: config.sshUser,
      sshPort: config.sshPort,
      workRoot: config.workRoot,
      keep: config.keep,
      state: "active",
      createdAt: now.toISOString(),
      updatedAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + config.ttlSeconds * 1000).toISOString(),
    };
    if (config.provider === "aws") {
      record.region = config.awsRegion;
    }
    await this.putLease(record);
    await this.scheduleAlarm();
    return json({ lease: record }, { status: 201 });
  }

  private async leaseRoute(request: Request, leaseID: string, action?: string): Promise<Response> {
    const method = request.method.toUpperCase();
    if (method === "GET" && action === undefined) {
      const lease = await this.getLease(leaseID);
      return lease ? json({ lease }) : json({ error: "not_found" }, { status: 404 });
    }
    if (method === "POST" && action === "heartbeat") {
      const lease = await this.requireLease(leaseID);
      lease.updatedAt = new Date().toISOString();
      await this.putLease(lease);
      return json({ lease });
    }
    if (method === "POST" && action === "release") {
      return this.releaseLease(request, leaseID);
    }
    return json({ error: "not_found" }, { status: 404 });
  }

  private async releaseLease(request: Request, leaseID: string): Promise<Response> {
    const lease = await this.requireLease(leaseID);
    const body = await optionalJson<{ delete?: boolean }>(request);
    const shouldDelete = body.delete ?? !lease.keep;
    if (shouldDelete && lease.state === "active") {
      await this.deleteLeaseServer(lease);
    }
    lease.state = "released";
    lease.updatedAt = new Date().toISOString();
    await this.putLease(lease);
    return json({ lease });
  }

  private async pool(request: Request): Promise<Response> {
    const url = new URL(request.url);
    const provider = url.searchParams.get("provider");
    const machines =
      provider === "aws"
        ? await this.provider("aws").listCrabboxServers()
        : provider === "hetzner"
          ? await this.provider("hetzner").listCrabboxServers()
          : [
              ...(await this.provider("hetzner").listCrabboxServers()),
              ...(await this.provider("aws")
                .listCrabboxServers()
                .catch(() => [])),
            ];
    return json({ machines });
  }

  private async listLeases(): Promise<Response> {
    const leases = await this.state.storage.list<LeaseRecord>({ prefix: "lease:" });
    return json({ leases: [...leases.values()] });
  }

  private async expireLeases(): Promise<void> {
    const leases = await this.state.storage.list<LeaseRecord>({ prefix: "lease:" });
    const now = Date.now();
    const expired = [...leases.values()].filter(
      (lease) => lease.state === "active" && Date.parse(lease.expiresAt) <= now,
    );
    await Promise.all(
      expired.map(async (lease) => {
        if (!lease.keep) {
          await this.deleteLeaseServer(lease).catch(() => undefined);
        }
        lease.state = "expired";
        lease.updatedAt = new Date().toISOString();
        await this.putLease(lease);
      }),
    );
  }

  private async scheduleAlarm(): Promise<void> {
    const leases = await this.state.storage.list<LeaseRecord>({ prefix: "lease:" });
    const activeExpiries = [...leases.values()]
      .filter((lease) => lease.state === "active")
      .map((lease) => Date.parse(lease.expiresAt))
      .filter((time) => Number.isFinite(time));
    if (activeExpiries.length === 0) {
      await this.state.storage.deleteAlarm();
      return;
    }
    await this.state.storage.setAlarm(Math.min(...activeExpiries));
  }

  private async getLease(leaseID: string): Promise<LeaseRecord | undefined> {
    return this.state.storage.get<LeaseRecord>(leaseKey(leaseID));
  }

  private async requireLease(leaseID: string): Promise<LeaseRecord> {
    const lease = await this.getLease(leaseID);
    if (!lease) {
      throw new Error(`lease not found: ${leaseID}`);
    }
    return lease;
  }

  private async putLease(lease: LeaseRecord): Promise<void> {
    await this.state.storage.put(leaseKey(lease.id), lease);
  }

  private provider(provider: Provider, region = "eu-west-1"): CloudProvider {
    if (provider === "aws") {
      return new AWSProvider(this.env, region || this.env.CRABBOX_AWS_REGION || "eu-west-1");
    }
    return new HetznerProvider(this.env);
  }

  private async deleteLeaseServer(lease: LeaseRecord): Promise<void> {
    if (lease.provider === "aws") {
      await this.provider("aws", lease.region).deleteServer(lease.cloudID);
      return;
    }
    await this.provider("hetzner").deleteServer(String(lease.serverID));
  }
}

function leaseKey(leaseID: string): string {
  return `lease:${leaseID}`;
}

function newLeaseID(): string {
  const bytes = new Uint8Array(6);
  crypto.getRandomValues(bytes);
  return `cbx_${[...bytes].map((byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

async function optionalJson<T>(request: Request): Promise<T> {
  if (!request.headers.get("content-type")?.includes("application/json")) {
    return {} as T;
  }
  return readJson<T>(request);
}

interface CloudProvider {
  listCrabboxServers(): Promise<ProviderMachine[]>;
  createServerWithFallback(
    config: ReturnType<typeof leaseConfig>,
    leaseID: string,
    owner: string,
  ): Promise<{ server: ProviderMachine; serverType: string }>;
  deleteServer(id: string): Promise<void>;
}

class HetznerProvider implements CloudProvider {
  private readonly client: HetznerClient;

  constructor(env: Env) {
    this.client = new HetznerClient(env);
  }

  async listCrabboxServers(): Promise<ProviderMachine[]> {
    const servers = await this.client.listCrabboxServers();
    return servers.map((server) => this.client.toMachine(server));
  }

  async createServerWithFallback(
    config: ReturnType<typeof leaseConfig>,
    leaseID: string,
    owner: string,
  ): Promise<{ server: ProviderMachine; serverType: string }> {
    const { server, serverType } = await this.client.createServerWithFallback(
      config,
      leaseID,
      owner,
    );
    return { server: this.client.toMachine(server), serverType };
  }

  async deleteServer(id: string): Promise<void> {
    await this.client.deleteServer(Number(id));
  }
}

class AWSProvider implements CloudProvider {
  private readonly client: EC2SpotClient;

  constructor(env: Env, region: string) {
    this.client = new EC2SpotClient(env, region);
  }

  listCrabboxServers(): Promise<ProviderMachine[]> {
    return this.client.listCrabboxServers();
  }

  async createServerWithFallback(
    config: ReturnType<typeof leaseConfig>,
    leaseID: string,
    owner: string,
  ): Promise<{ server: ProviderMachine; serverType: string }> {
    const { server, serverType } = await this.client.createServerWithFallback(
      config,
      leaseID,
      owner,
    );
    return { server: await this.client.waitForServerIP(server.cloudID), serverType };
  }

  async deleteServer(id: string): Promise<void> {
    await this.client.deleteServer(id);
  }
}
