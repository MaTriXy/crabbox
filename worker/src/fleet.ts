import { EC2SpotClient } from "./aws";
import { leaseConfig } from "./config";
import { HetznerClient } from "./hetzner";
import { errorMessage, json, pathParts, readJson, requestOwner } from "./http";
import type {
  Env,
  LeaseRecord,
  LeaseRequest,
  Provider,
  ProviderMachine,
  RunCreateRequest,
  RunFinishRequest,
  RunRecord,
} from "./types";
import { costLimits, enforceCostLimits, leaseCost, requestOrg, usageSummary } from "./usage";

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
      if (method === "GET" && parts.join("/") === "v1/usage") {
        return await this.usage(request);
      }
      if (method === "GET" && parts.join("/") === "v1/whoami") {
        return this.whoami(request);
      }
      if (method === "GET" && parts.join("/") === "v1/admin/leases") {
        return await this.adminLeases(request);
      }
      if (parts[0] === "v1" && parts[1] === "admin" && parts[2] === "leases" && parts[3]) {
        return await this.adminLeaseRoute(request, parts[3], parts[4]);
      }
      if (method === "GET" && parts.join("/") === "v1/runs") {
        return await this.listRuns(request);
      }
      if (method === "POST" && parts.join("/") === "v1/runs") {
        return await this.createRun(request);
      }
      if (parts[0] === "v1" && parts[1] === "runs" && parts[2]) {
        return await this.runRoute(request, parts[2], parts[3]);
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
    const org = requestOrg(request, this.env);
    const input = await readJson<LeaseRequest>(request);
    const config = leaseConfig(input);
    const leaseID = validLeaseID(input.leaseID) ? input.leaseID : newLeaseID();
    const provider = this.provider(config.provider, config.awsRegion);
    const providerHourlyUSD = await provider
      .hourlyPriceUSD(config.serverType, config)
      .catch(() => undefined);
    const cost = leaseCost(
      this.env,
      config.provider,
      config.serverType,
      config.ttlSeconds,
      providerHourlyUSD,
    );
    const now = new Date();
    const record: LeaseRecord = {
      id: leaseID,
      provider: config.provider,
      cloudID: "",
      owner,
      org,
      profile: config.profile,
      class: config.class,
      serverType: config.serverType,
      serverID: 0,
      serverName: "",
      providerKey: config.providerKey,
      host: "",
      sshUser: config.sshUser,
      sshPort: config.sshPort,
      workRoot: config.workRoot,
      keep: config.keep,
      ttlSeconds: config.ttlSeconds,
      estimatedHourlyUSD: cost.hourlyUSD,
      maxEstimatedUSD: cost.maxUSD,
      state: "active",
      createdAt: now.toISOString(),
      updatedAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + config.ttlSeconds * 1000).toISOString(),
    };
    const leases = await this.leaseRecords();
    const limitError = enforceCostLimits(leases, record, costLimits(this.env), now);
    if (limitError) {
      return json({ error: "cost_limit_exceeded", message: limitError }, { status: 429 });
    }
    const { server, serverType } = await provider.createServerWithFallback(config, leaseID, owner);
    record.cloudID = server.cloudID;
    record.serverType = serverType;
    record.serverID = server.id;
    record.serverName = server.name;
    record.host = server.host;
    const finalProviderHourlyUSD = await provider
      .hourlyPriceUSD(serverType, config)
      .catch(() => undefined);
    const finalCost = leaseCost(
      this.env,
      config.provider,
      serverType,
      config.ttlSeconds,
      finalProviderHourlyUSD,
    );
    record.estimatedHourlyUSD = finalCost.hourlyUSD;
    record.maxEstimatedUSD = finalCost.maxUSD;
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
      const now = new Date();
      lease.updatedAt = now.toISOString();
      lease.expiresAt = new Date(now.getTime() + leaseTTLSeconds(lease) * 1000).toISOString();
      await this.putLease(lease);
      await this.scheduleAlarm();
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
    const now = new Date().toISOString();
    lease.state = "released";
    lease.updatedAt = now;
    lease.releasedAt = now;
    lease.endedAt = now;
    await this.putLease(lease);
    return json({ lease });
  }

  private whoami(request: Request): Response {
    return json({
      owner: requestOwner(request),
      org: requestOrg(request, this.env),
      auth: "bearer",
    });
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
    return json({ leases: await this.leaseRecords() });
  }

  private async adminLeases(request: Request): Promise<Response> {
    return json({ leases: this.filterLeases(await this.leaseRecords(), request) });
  }

  private async adminLeaseRoute(
    request: Request,
    leaseID: string,
    action?: string,
  ): Promise<Response> {
    if (request.method.toUpperCase() !== "POST") {
      return json({ error: "not_found" }, { status: 404 });
    }
    if (action === "release") {
      return this.releaseLease(request, leaseID);
    }
    if (action === "delete") {
      return this.adminDeleteLease(leaseID);
    }
    return json({ error: "not_found" }, { status: 404 });
  }

  private async adminDeleteLease(leaseID: string): Promise<Response> {
    const lease = await this.requireLease(leaseID);
    if (lease.state === "active") {
      await this.deleteLeaseServer(lease);
    }
    const now = new Date().toISOString();
    lease.state = "released";
    lease.updatedAt = now;
    lease.releasedAt = now;
    lease.endedAt = now;
    lease.keep = false;
    await this.putLease(lease);
    return json({ lease });
  }

  private filterLeases(leases: LeaseRecord[], request: Request): LeaseRecord[] {
    const url = new URL(request.url);
    const state = url.searchParams.get("state") ?? "";
    const owner = url.searchParams.get("owner") ?? "";
    const org = url.searchParams.get("org") ?? "";
    const limit = clampLimit(url.searchParams.get("limit"), 100);
    return leases
      .filter((lease) => !state || lease.state === state)
      .filter((lease) => !owner || lease.owner === owner)
      .filter((lease) => !org || lease.org === org)
      .toSorted((a, b) => b.createdAt.localeCompare(a.createdAt))
      .slice(0, limit);
  }

  private async createRun(request: Request): Promise<Response> {
    const owner = requestOwner(request);
    const org = requestOrg(request, this.env);
    const input = await readJson<RunCreateRequest>(request);
    if (!validLeaseID(input.leaseID)) {
      return json({ error: "invalid_lease_id" }, { status: 400 });
    }
    const lease = await this.getLease(input.leaseID);
    const now = new Date().toISOString();
    const run: RunRecord = {
      id: newRunID(),
      leaseID: input.leaseID,
      owner,
      org,
      provider: input.provider ?? lease?.provider ?? "hetzner",
      class: input.class ?? lease?.class ?? "",
      serverType: input.serverType ?? lease?.serverType ?? "",
      command: Array.isArray(input.command) ? input.command.map(String) : [],
      state: "running",
      logBytes: 0,
      logTruncated: false,
      startedAt: now,
    };
    await this.putRun(run);
    return json({ run }, { status: 201 });
  }

  private async runRoute(request: Request, runID: string, action?: string): Promise<Response> {
    const method = request.method.toUpperCase();
    if (method === "GET" && action === undefined) {
      const run = await this.getRun(runID);
      return run ? json({ run }) : json({ error: "not_found" }, { status: 404 });
    }
    if (method === "GET" && action === "logs") {
      const run = await this.getRun(runID);
      if (!run) {
        return json({ error: "not_found" }, { status: 404 });
      }
      const log = (await this.state.storage.get<string>(runLogKey(runID))) ?? "";
      return new Response(log, {
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    }
    if (method === "POST" && action === "finish") {
      return this.finishRun(request, runID);
    }
    return json({ error: "not_found" }, { status: 404 });
  }

  private async finishRun(request: Request, runID: string): Promise<Response> {
    const run = await this.requireRun(runID);
    const input = await readJson<RunFinishRequest>(request);
    const now = new Date();
    const started = Date.parse(run.startedAt);
    run.exitCode = Number.isFinite(input.exitCode) ? input.exitCode : 1;
    const syncMs = finiteNumber(input.syncMs);
    const commandMs = finiteNumber(input.commandMs);
    if (syncMs !== undefined) {
      run.syncMs = syncMs;
    }
    if (commandMs !== undefined) {
      run.commandMs = commandMs;
    }
    if (Number.isFinite(started)) {
      run.durationMs = now.getTime() - started;
    }
    run.state = run.exitCode === 0 ? "succeeded" : "failed";
    run.endedAt = now.toISOString();
    const log = input.log ?? "";
    run.logBytes = new TextEncoder().encode(log).byteLength;
    run.logTruncated = Boolean(input.logTruncated);
    if (input.results) {
      run.results = input.results;
    }
    await this.state.storage.put(runLogKey(runID), log);
    await this.putRun(run);
    return json({ run });
  }

  private async listRuns(request: Request): Promise<Response> {
    const url = new URL(request.url);
    const leaseID = url.searchParams.get("leaseID") ?? "";
    const owner = url.searchParams.get("owner") ?? "";
    const org = url.searchParams.get("org") ?? "";
    const state = url.searchParams.get("state") ?? "";
    const limit = clampLimit(url.searchParams.get("limit"), 50);
    const runs = await this.runRecords();
    return json({
      runs: runs
        .filter((run) => !leaseID || run.leaseID === leaseID)
        .filter((run) => !owner || run.owner === owner)
        .filter((run) => !org || run.org === org)
        .filter((run) => !state || run.state === state)
        .toSorted((a, b) => b.startedAt.localeCompare(a.startedAt))
        .slice(0, limit),
    });
  }

  private async usage(request: Request): Promise<Response> {
    const url = new URL(request.url);
    const requestedScope = url.searchParams.get("scope") ?? "user";
    const scope =
      requestedScope === "org" || requestedScope === "all" || requestedScope === "user"
        ? requestedScope
        : "user";
    const month = url.searchParams.get("month") ?? new Date().toISOString().slice(0, 7);
    const owner = url.searchParams.get("owner") ?? requestOwner(request);
    const org = url.searchParams.get("org") ?? requestOrg(request, this.env);
    const usage = usageSummary(await this.leaseRecords(), { scope, owner, org, month }, new Date());
    return json({ usage, limits: costLimits(this.env) });
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
        const nowISO = new Date().toISOString();
        lease.state = "expired";
        lease.updatedAt = nowISO;
        lease.endedAt = nowISO;
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

  private async leaseRecords(): Promise<LeaseRecord[]> {
    const leases = await this.state.storage.list<LeaseRecord>({ prefix: "lease:" });
    return [...leases.values()];
  }

  private async runRecords(): Promise<RunRecord[]> {
    const runs = await this.state.storage.list<RunRecord>({ prefix: "run:" });
    return [...runs.values()];
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

  private async getRun(runID: string): Promise<RunRecord | undefined> {
    return this.state.storage.get<RunRecord>(runKey(runID));
  }

  private async requireRun(runID: string): Promise<RunRecord> {
    const run = await this.getRun(runID);
    if (!run) {
      throw new Error(`run not found: ${runID}`);
    }
    return run;
  }

  private async putRun(run: RunRecord): Promise<void> {
    await this.state.storage.put(runKey(run.id), run);
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
      if (validCrabboxProviderKey(lease.providerKey)) {
        await this.provider("aws", lease.region).deleteSSHKey(lease.providerKey);
      }
      return;
    }
    await this.provider("hetzner").deleteServer(String(lease.serverID));
    if (validCrabboxProviderKey(lease.providerKey)) {
      await this.provider("hetzner").deleteSSHKey(lease.providerKey);
    }
  }
}

function leaseKey(leaseID: string): string {
  return `lease:${leaseID}`;
}

function runKey(runID: string): string {
  return `run:${runID}`;
}

function runLogKey(runID: string): string {
  return `runlog:${runID}`;
}

function newLeaseID(): string {
  const bytes = new Uint8Array(6);
  crypto.getRandomValues(bytes);
  return `cbx_${[...bytes].map((byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

function newRunID(): string {
  const bytes = new Uint8Array(6);
  crypto.getRandomValues(bytes);
  return `run_${[...bytes].map((byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

function validLeaseID(value: string | undefined): value is string {
  return typeof value === "string" && /^cbx_[a-f0-9]{12}$/.test(value);
}

function validCrabboxProviderKey(value: string | undefined): value is string {
  return typeof value === "string" && /^crabbox-cbx-[a-f0-9]{12}$/.test(value);
}

function clampLimit(value: string | null, fallback: number): number {
  const parsed = Number(value ?? "");
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return Math.min(Math.trunc(parsed), 500);
}

function finiteNumber(value: number | undefined): number | undefined {
  return Number.isFinite(value) ? value : undefined;
}

function leaseTTLSeconds(lease: LeaseRecord): number {
  if (Number.isFinite(lease.ttlSeconds) && lease.ttlSeconds > 0) {
    return lease.ttlSeconds;
  }
  const createdAt = Date.parse(lease.createdAt);
  const expiresAt = Date.parse(lease.expiresAt);
  if (Number.isFinite(createdAt) && Number.isFinite(expiresAt) && expiresAt > createdAt) {
    return Math.min(Math.trunc((expiresAt - createdAt) / 1000), 86_400);
  }
  return 5_400;
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
  deleteSSHKey(name: string): Promise<void>;
  hourlyPriceUSD(
    serverType: string,
    config: ReturnType<typeof leaseConfig>,
  ): Promise<number | undefined>;
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

  async deleteSSHKey(name: string): Promise<void> {
    await this.client.deleteSSHKey(name);
  }

  hourlyPriceUSD(
    serverType: string,
    config: ReturnType<typeof leaseConfig>,
  ): Promise<number | undefined> {
    return this.client.hourlyPriceUSD(serverType, config.location);
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

  async deleteSSHKey(name: string): Promise<void> {
    await this.client.deleteSSHKey(name);
  }

  hourlyPriceUSD(serverType: string): Promise<number | undefined> {
    return this.client.hourlySpotPriceUSD(serverType);
  }
}
