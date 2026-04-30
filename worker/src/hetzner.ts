import { cloudInit } from "./bootstrap";
import { serverTypeCandidatesForClass, type LeaseConfig } from "./config";
import type { Env, HetznerSSHKey, HetznerServer, MachineView } from "./types";

interface HetznerListServersResponse {
  servers: HetznerServer[];
}

interface HetznerListSSHKeysResponse {
  ssh_keys: HetznerSSHKey[];
}

interface HetznerSSHKeyResponse {
  ssh_key: HetznerSSHKey;
}

interface HetznerServerResponse {
  server: HetznerServer;
}

export class HetznerClient {
  private readonly token: string;

  constructor(env: Env) {
    if (!env.HETZNER_TOKEN) {
      throw new Error("HETZNER_TOKEN secret is required");
    }
    this.token = env.HETZNER_TOKEN;
  }

  async listCrabboxServers(): Promise<HetznerServer[]> {
    const query = new URLSearchParams({
      label_selector: "crabbox=true",
      per_page: "100",
    });
    const response = await this.request<HetznerListServersResponse>("GET", `/servers?${query}`);
    return response.servers;
  }

  async ensureSSHKey(name: string, publicKey: string): Promise<HetznerSSHKey> {
    const byName = await this.request<HetznerListSSHKeysResponse>(
      "GET",
      `/ssh_keys?${new URLSearchParams({ name })}`,
    );
    for (const key of byName.ssh_keys) {
      if (key.name === name) {
        if (key.public_key.trim() !== publicKey.trim()) {
          throw new Error(`hetzner ssh key ${name} exists with different public key`);
        }
        return key;
      }
    }

    const all = await this.request<HetznerListSSHKeysResponse>(
      "GET",
      `/ssh_keys?${new URLSearchParams({ per_page: "100" })}`,
    );
    for (const key of all.ssh_keys) {
      if (key.public_key.trim() === publicKey.trim()) {
        return key;
      }
    }

    const created = await this.request<HetznerSSHKeyResponse>("POST", "/ssh_keys", {
      name,
      public_key: publicKey,
      labels: {
        crabbox: "true",
        created_by: "crabbox",
      },
    });
    return created.ssh_key;
  }

  async createServerWithFallback(
    config: LeaseConfig,
    leaseID: string,
    owner: string,
  ): Promise<{ server: HetznerServer; serverType: string }> {
    const key = await this.ensureSSHKey(config.providerKey, config.sshPublicKey);
    const resolvedConfig = { ...config, providerKey: key.name };
    const candidates = prependUnique(
      resolvedConfig.serverType,
      serverTypeCandidatesForClass(resolvedConfig.class),
    );
    const failures: string[] = [];
    for (const serverType of candidates) {
      try {
        // oxlint-disable-next-line eslint/no-await-in-loop -- server-type fallback must stay sequential.
        const server = await this.createServer({ ...resolvedConfig, serverType }, leaseID, owner);
        return { server, serverType };
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        failures.push(`${serverType}: ${message}`);
        if (!isRetryableProvisioningError(message)) {
          break;
        }
      }
    }
    throw new Error(failures.join("; "));
  }

  async getServer(id: number): Promise<HetznerServer> {
    return (await this.request<HetznerServerResponse>("GET", `/servers/${id}`)).server;
  }

  async waitForServerIP(id: number): Promise<HetznerServer> {
    const deadline = Date.now() + 60_000;
    while (Date.now() < deadline) {
      // oxlint-disable-next-line eslint/no-await-in-loop -- polling must wait between Hetzner API reads.
      const server = await this.getServer(id);
      if (server.public_net.ipv4.ip) {
        return server;
      }
      // oxlint-disable-next-line eslint/no-await-in-loop -- this delay is the polling interval.
      await sleep(2_000);
    }
    throw new Error(`timed out waiting for server IP: ${id}`);
  }

  async deleteServer(id: number): Promise<void> {
    await this.request<void>("DELETE", `/servers/${id}`);
  }

  toMachine(server: HetznerServer): MachineView {
    return {
      id: server.id,
      name: server.name,
      status: server.status,
      serverType: server.server_type.name,
      host: server.public_net.ipv4.ip,
      labels: server.labels,
    };
  }

  private async createServer(
    config: LeaseConfig,
    leaseID: string,
    owner: string,
  ): Promise<HetznerServer> {
    const name = `crabbox-${leaseID}`.replaceAll("_", "-");
    const labels = {
      crabbox: "true",
      profile: config.profile,
      class: config.class,
      server_type: config.serverType,
      lease: leaseID,
      state: "leased",
      keep: String(config.keep),
      owner: sanitizeLabel(owner),
      created_by: "crabbox",
    };
    const response = await this.request<HetznerServerResponse>("POST", "/servers", {
      name,
      server_type: config.serverType,
      image: config.image,
      location: config.location,
      labels,
      ssh_keys: [config.providerKey],
      user_data: cloudInit(config),
      start_after_create: true,
      public_net: {
        enable_ipv4: true,
        enable_ipv6: false,
      },
    });
    return response.server.public_net.ipv4.ip
      ? response.server
      : await this.waitForServerIP(response.server.id);
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const init: RequestInit = {
      method,
      headers: {
        authorization: `Bearer ${this.token}`,
        "content-type": "application/json",
      },
    };
    if (body !== undefined) {
      init.body = JSON.stringify(body);
    }
    const response = await fetch(`https://api.hetzner.cloud/v1${path}`, init);
    if (!response.ok) {
      throw new Error(
        `hetzner ${method} ${path}: http ${response.status}: ${await safeBody(response)}`,
      );
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }
}

export function isRetryableProvisioningError(message: string): boolean {
  return (
    message.includes("dedicated_core_limit") ||
    message.includes("resource_limit_exceeded") ||
    message.includes("server_type_not_available") ||
    message.includes("location_not_available")
  );
}

function prependUnique(first: string, rest: string[]): string[] {
  return [first, ...rest.filter((value) => value !== first)];
}

function sanitizeLabel(value: string): string {
  return value.replaceAll(/[^a-zA-Z0-9_.@-]/g, "_").slice(0, 63) || "unknown";
}

async function safeBody(response: Response): Promise<string> {
  const text = await response.text();
  return text.length > 500 ? `${text.slice(0, 500)}...` : text;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
