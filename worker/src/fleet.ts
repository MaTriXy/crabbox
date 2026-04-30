import { leaseConfig } from "./config";
import { HetznerClient } from "./hetzner";
import { errorMessage, json, pathParts, readJson, requestOwner } from "./http";
import type { Env, LeaseRecord, LeaseRequest } from "./types";

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
        return await this.pool();
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
    const client = new HetznerClient(this.env);
    const { server, serverType } = await client.createServerWithFallback(config, leaseID, owner);
    const now = new Date();
    const record: LeaseRecord = {
      id: leaseID,
      owner,
      profile: config.profile,
      class: config.class,
      serverType,
      serverID: server.id,
      serverName: server.name,
      host: server.public_net.ipv4.ip,
      sshUser: config.sshUser,
      sshPort: config.sshPort,
      workRoot: config.workRoot,
      keep: config.keep,
      state: "active",
      createdAt: now.toISOString(),
      updatedAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + config.ttlSeconds * 1000).toISOString(),
    };
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
      await new HetznerClient(this.env).deleteServer(lease.serverID);
    }
    lease.state = "released";
    lease.updatedAt = new Date().toISOString();
    await this.putLease(lease);
    return json({ lease });
  }

  private async pool(): Promise<Response> {
    const client = new HetznerClient(this.env);
    const servers = await client.listCrabboxServers();
    return json({ machines: servers.map((server) => client.toMachine(server)) });
  }

  private async listLeases(): Promise<Response> {
    const leases = await this.state.storage.list<LeaseRecord>({ prefix: "lease:" });
    return json({ leases: [...leases.values()] });
  }

  private async expireLeases(): Promise<void> {
    const leases = await this.state.storage.list<LeaseRecord>({ prefix: "lease:" });
    const now = Date.now();
    const client = new HetznerClient(this.env);
    const expired = [...leases.values()].filter(
      (lease) => lease.state === "active" && Date.parse(lease.expiresAt) <= now,
    );
    await Promise.all(
      expired.map(async (lease) => {
        if (!lease.keep) {
          await client.deleteServer(lease.serverID).catch(() => undefined);
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
