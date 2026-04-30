import type { LeaseRequest } from "./types";

export interface LeaseConfig {
  profile: string;
  class: string;
  serverType: string;
  location: string;
  image: string;
  sshUser: string;
  sshPort: string;
  providerKey: string;
  workRoot: string;
  ttlSeconds: number;
  keep: boolean;
  sshPublicKey: string;
}

export function leaseConfig(input: LeaseRequest): LeaseConfig {
  const machineClass = input.class ?? "beast";
  const serverType = input.serverType ?? serverTypeForClass(machineClass);
  const ttlSeconds = clampTTL(input.ttlSeconds ?? 5400);
  const sshPublicKey = input.sshPublicKey?.trim() ?? "";
  if (!sshPublicKey) {
    throw new Error("sshPublicKey is required");
  }
  return {
    profile: input.profile ?? "openclaw-check",
    class: machineClass,
    serverType,
    location: input.location ?? "fsn1",
    image: input.image ?? "ubuntu-24.04",
    sshUser: input.sshUser ?? "crabbox",
    sshPort: input.sshPort ?? "2222",
    providerKey: input.providerKey ?? "crabbox-steipete",
    workRoot: input.workRoot ?? "/work/crabbox",
    ttlSeconds,
    keep: input.keep ?? false,
    sshPublicKey,
  };
}

export function serverTypeForClass(machineClass: string): string {
  return serverTypeCandidatesForClass(machineClass)[0] ?? machineClass;
}

export function serverTypeCandidatesForClass(machineClass: string): string[] {
  switch (machineClass) {
    case "standard":
      return ["ccx33", "cpx62", "cx53"];
    case "fast":
      return ["ccx43", "cpx62", "cx53"];
    case "large":
      return ["ccx53", "ccx43", "cpx62", "cx53"];
    case "beast":
      return ["ccx63", "ccx53", "ccx43", "cpx62", "cx53"];
    default:
      return [machineClass];
  }
}

function clampTTL(ttlSeconds: number): number {
  if (!Number.isFinite(ttlSeconds) || ttlSeconds <= 0) {
    return 5400;
  }
  return Math.min(Math.trunc(ttlSeconds), 86_400);
}
