import type { LeaseRequest, Provider } from "./types";

export interface LeaseConfig {
  provider: Provider;
  profile: string;
  class: string;
  serverType: string;
  location: string;
  image: string;
  awsRegion: string;
  awsAMI: string;
  awsSGID: string;
  awsSubnetID: string;
  awsProfile: string;
  awsRootGB: number;
  sshUser: string;
  sshPort: string;
  providerKey: string;
  workRoot: string;
  ttlSeconds: number;
  keep: boolean;
  sshPublicKey: string;
}

export function leaseConfig(input: LeaseRequest): LeaseConfig {
  const provider = input.provider ?? "hetzner";
  if (provider !== "hetzner" && provider !== "aws") {
    throw new Error(`unsupported provider: ${String(provider)}`);
  }
  const machineClass = input.class ?? "beast";
  const serverType = input.serverType ?? serverTypeForProviderClass(provider, machineClass);
  const ttlSeconds = clampTTL(input.ttlSeconds ?? 5400);
  const sshPublicKey = input.sshPublicKey?.trim() ?? "";
  if (!sshPublicKey) {
    throw new Error("sshPublicKey is required");
  }
  return {
    provider,
    profile: input.profile ?? "openclaw-check",
    class: machineClass,
    serverType,
    location: input.location ?? "fsn1",
    image: input.image ?? "ubuntu-24.04",
    awsRegion: input.awsRegion ?? "eu-west-1",
    awsAMI: input.awsAMI ?? "",
    awsSGID: input.awsSGID ?? "",
    awsSubnetID: input.awsSubnetID ?? "",
    awsProfile: input.awsProfile ?? "",
    awsRootGB: input.awsRootGB ?? 400,
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

export function serverTypeForProviderClass(provider: Provider, machineClass: string): string {
  if (provider === "aws") {
    return awsInstanceTypeCandidatesForClass(machineClass)[0] ?? machineClass;
  }
  return serverTypeForClass(machineClass);
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

export function awsInstanceTypeCandidatesForClass(machineClass: string): string[] {
  switch (machineClass) {
    case "standard":
      return ["c7a.8xlarge", "c7a.4xlarge"];
    case "fast":
      return ["c7a.16xlarge", "c7a.12xlarge", "c7a.8xlarge"];
    case "large":
      return ["c7a.24xlarge", "c7a.16xlarge", "c7a.12xlarge"];
    case "beast":
      return ["c7a.48xlarge", "c7a.32xlarge", "c7a.24xlarge", "c7a.16xlarge"];
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
