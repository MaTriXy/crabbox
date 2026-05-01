import { describe, expect, it } from "vitest";

import { cloudInit } from "../src/bootstrap";
import type { LeaseConfig } from "../src/config";

const config: LeaseConfig = {
  provider: "aws",
  profile: "project-check",
  class: "standard",
  serverType: "c7a.8xlarge",
  location: "fsn1",
  image: "ubuntu-24.04",
  awsRegion: "eu-west-1",
  awsAMI: "",
  awsSGID: "",
  awsSubnetID: "",
  awsProfile: "",
  awsRootGB: 400,
  capacityMarket: "spot",
  capacityStrategy: "most-available",
  capacityFallback: "on-demand-after-120s",
  capacityRegions: [],
  capacityAvailabilityZones: [],
  sshUser: "crabbox",
  sshPort: "2222",
  providerKey: "crabbox-steipete",
  workRoot: "/work/crabbox",
  ttlSeconds: 1200,
  keep: false,
  sshPublicKey: "ssh-ed25519 test",
};

describe("cloud-init bootstrap", () => {
  it("uses retrying package installation in runcmd", () => {
    const got = cloudInit(config);
    expect(got).toContain("package_update: false");
    expect(got).toContain("bash -euxo pipefail <<'BOOT'");
    expect(got).toContain('Acquire::Retries "8";');
    expect(got).toContain("retry apt-get update");
    expect(got).toContain(
      "retry apt-get install -y --no-install-recommends openssh-server ca-certificates curl git rsync build-essential docker.io jq",
    );
    expect(got).toContain("https://deb.nodesource.com/setup_24.x");
    expect(got).toContain("retry corepack prepare pnpm@10.33.2 --activate");
    expect(got).toContain("docker --version");
    expect(got).not.toContain("\npackages:\n");
  });
});
