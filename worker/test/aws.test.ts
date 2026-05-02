import { describe, expect, it } from "vitest";

import { createSecurityGroupParams } from "../src/aws";

describe("aws provider", () => {
  it("uses the EC2 query parameter names for security group creation", () => {
    const params = createSecurityGroupParams("crabbox-runners", "vpc-123");

    expect(params).toMatchObject({
      GroupDescription: "Crabbox ephemeral test runners",
      GroupName: "crabbox-runners",
      VpcId: "vpc-123",
      "TagSpecification.1.ResourceType": "security-group",
      "TagSpecification.1.Tag.1.Key": "Name",
      "TagSpecification.1.Tag.1.Value": "crabbox-runners",
      "TagSpecification.1.Tag.2.Key": "crabbox",
      "TagSpecification.1.Tag.2.Value": "true",
      "TagSpecification.1.Tag.3.Key": "created_by",
      "TagSpecification.1.Tag.3.Value": "crabbox",
    });
    expect(params).not.toHaveProperty("Description");
  });
});
