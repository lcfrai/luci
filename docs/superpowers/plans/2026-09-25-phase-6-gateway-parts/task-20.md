### Task 20: Bedrock federation module (AWS), never applied

**The AU route's credential, with no long-lived secret anywhere.** LiteLLM's AU group calls Amazon Bedrock in `ap-southeast-2`. The obvious way to give it a credential, an AWS access key in Key Vault, is exactly the "usable long-lived secret in the cloud" global constraint 2 forbids. This module writes the other way: AWS trusts the AKS cluster's OIDC issuer, and exactly one Kubernetes service account, `panacea-gateway/litellm`, may exchange its projected token (audience `sts.amazonaws.com`, task 19's decision (b)) for a one-hour role session. The role may invoke Anthropic models through `au.` inference profiles in Australia and nothing else, and an explicit deny refuses any request to a region outside Australia.

**Written and tested with mocks; never applied.** There is no AWS account yet (owner question Q3). No root stack in this repository calls the module; the stack that does is phase 8's to write once the account exists, with the cluster's issuer from phase 8's foundation output `aks_oidc_issuer_url` (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-07.md:346`, from `modules/aks`'s `oidc_issuer_url`, `task-06.md:171-173`). So ask P8-5 is already met by phase 8's plan as written; what remains is the calling stack and its AWS provider configuration.

**The `au.` profiles and the two regions are UNVERIFIED.** 05-design records that *"Bedrock's `au.` profiles reportedly span Sydney and Melbourne, while `apac.` and `global.` leave Australia"*, from search summaries only (`docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:225-227`). The module therefore allows `ap-southeast-2` and `ap-southeast-4` and no other region, as a validated variable, and the tests fail on any ARN that names an `apac.` or `global.` profile. If task 24's P04 shows an `au.` profile routing somewhere else, the deny refuses it, which is the safe failure.

**Decisions this part makes:**
- **(a) Policies are built with `jsonencode`, not `aws_iam_policy_document`.** Under `mock_provider "aws"`, a data source returns mock values, so a policy document built by one could not be asserted on. `jsonencode` of a literal structure is computed by Terraform itself and survives mocking byte for byte.
- **(b) The account id is a variable**, validated as twelve digits, rather than `aws_caller_identity`, for the same reason and because the owner names the account (Q3).
- **(c) The subject is matched with `StringEquals`, never `StringLike`.** One service account, one namespace, no wildcard.
- **(d) The issuer's trailing slash.** AKS issuer URLs end in `/` (training knowledge, **UNVERIFIED**). The IAM provider is created with the URL as given, and the condition keys use the host and path with `https://` and the trailing `/` removed, which is how IAM names OIDC condition keys (training knowledge, **UNVERIFIED**; task 24's P04 is the first real exchange).

**Files:**
- `infra/terraform/modules/bedrock-federation/versions.tf` (new)
- `infra/terraform/modules/bedrock-federation/variables.tf` (new)
- `infra/terraform/modules/bedrock-federation/main.tf` (new)
- `infra/terraform/modules/bedrock-federation/outputs.tf` (new)
- `infra/terraform/modules/bedrock-federation/tests/federation.tftest.hcl` (new)

`infra/terraform/` already holds a root configuration (`providers.tf`, `kata.tf`, `variables.tf`, `dev.tfvars`); this module sits under `modules/` beside it, is not referenced from it, and so changes nothing that root plans.

**Interfaces:**
- Produces:
  - inputs `oidc_issuer_url` (string, `https://…`), `aws_account_id` (twelve digits), `namespace` (default `panacea-gateway`), `service_account` (default `litellm`), `regions` (default `["ap-southeast-2", "ap-southeast-4"]`, each validated against that pair), `role_name` (default `panacea-gateway-bedrock-au`), `tags` (map)
  - outputs `role_arn` (task 19's `bedrock.roleArn`), `oidc_provider_arn`, `allowed_regions`
- Consumes: task 02's AU upstream ids, which are `au.anthropic.<id>` profiles (task 02's catalogue test enforces the prefix); phase 8's `aks_oidc_issuer_url`.
- Consumed by: task 19 (`bedrock.roleArn`), task 24 (P04, the first real call), phase 8 (the calling stack).

**The two policies, complete, as the module renders them** (`<issuer>` is the issuer URL without `https://` and without a trailing `/`):

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Sid": "LiteLLMServiceAccountOnly",
    "Effect": "Allow",
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Principal": {"Federated": "<oidc provider arn>"},
    "Condition": {"StringEquals": {
      "<issuer>:sub": "system:serviceaccount:panacea-gateway:litellm",
      "<issuer>:aud": "sts.amazonaws.com"
    }}
  }]
}
```

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {"Sid": "InvokeAuProfilesOnly", "Effect": "Allow",
     "Action": ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"],
     "Resource": [
       "arn:aws:bedrock:ap-southeast-2:<account>:inference-profile/au.anthropic.*",
       "arn:aws:bedrock:ap-southeast-4:<account>:inference-profile/au.anthropic.*",
       "arn:aws:bedrock:ap-southeast-2::foundation-model/anthropic.*",
       "arn:aws:bedrock:ap-southeast-4::foundation-model/anthropic.*"]},
    {"Sid": "DenyOutsideAustralia", "Effect": "Deny", "Action": "*", "Resource": "*",
     "Condition": {"StringNotEquals": {"aws:RequestedRegion": ["ap-southeast-2", "ap-southeast-4"]}}}
  ]
}
```

The foundation-model ARNs are there because invoking through a cross-region profile is also authorised against the model in the region that serves it (Bedrock's documented behaviour as remembered, **UNVERIFIED**). The inference-profile ARN carrying the account id is likewise remembered, not read.

- [ ] **Step 1: Write the failing tests**

`infra/terraform/modules/bedrock-federation/tests/federation.tftest.hcl`, mocks only, in phase 8's style (`phase-8-own-environment.md:460`, *"terraform test, mock providers only"*). Each `run` names the mutant it kills. The runs use `command = apply` against the mock, so computed values such as the provider's ARN are known; nothing reaches AWS.

```hcl
# Bedrock federation module (phase 6, task 20). mock_provider only: no AWS call is made.
mock_provider "aws" {}

variables {
  oidc_issuer_url = "https://australiaeast.oic.prod-aks.azure.com/00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111/"
  aws_account_id  = "000000000000"
}

run "trust_is_exactly_one_service_account" {
  # Kills a StringLike or wildcard subject, a second statement, a missing audience,
  # and an account principal beside the federated one.
  assert {
    condition     = length(jsondecode(aws_iam_role.this.assume_role_policy).Statement) == 1
    error_message = "the trust policy must have exactly one statement"
  }
  assert {
    condition = jsondecode(aws_iam_role.this.assume_role_policy).Statement[0].Condition.StringEquals == {
      "australiaeast.oic.prod-aks.azure.com/00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111:sub" = "system:serviceaccount:panacea-gateway:litellm"
      "australiaeast.oic.prod-aks.azure.com/00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111:aud" = "sts.amazonaws.com"
    }
    error_message = "the trust must match sub and aud exactly, with the issuer's https:// and trailing / removed"
  }
  assert {
    condition     = !can(jsondecode(aws_iam_role.this.assume_role_policy).Statement[0].Condition.StringLike)
    error_message = "the trust must never use StringLike"
  }
  assert {
    condition     = jsondecode(aws_iam_role.this.assume_role_policy).Statement[0].Principal == { Federated = aws_iam_openid_connect_provider.this.arn }
    error_message = "the only principal is the cluster's OIDC provider"
  }
  assert {
    condition     = aws_iam_openid_connect_provider.this.client_id_list == ["sts.amazonaws.com"]
    error_message = "the OIDC provider's only audience is sts.amazonaws.com"
  }
  assert {
    condition     = aws_iam_role.this.max_session_duration == 3600
    error_message = "a role session lasts one hour"
  }
}

run "invoke_stays_in_australia" {
  # Kills an apac. or global. profile, a region outside the pair, bedrock:*, and a missing deny.
  assert {
    condition = alltrue([for a in jsondecode(aws_iam_role_policy.invoke.policy).Statement[0].Resource :
      !strcontains(a, "apac.") && !strcontains(a, "global.") &&
      contains(["ap-southeast-2", "ap-southeast-4"], split(":", a)[3]) &&
      (strcontains(a, ":inference-profile/au.anthropic.") || strcontains(a, "::foundation-model/anthropic."))])
    error_message = "every resource is an au.anthropic profile or an Anthropic model in Sydney or Melbourne"
  }
  assert {
    condition = toset(jsondecode(aws_iam_role_policy.invoke.policy).Statement[0].Action) == toset(
      ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"])
    error_message = "the role may invoke, and nothing else"
  }
  assert {
    condition = (jsondecode(aws_iam_role_policy.invoke.policy).Statement[1].Effect == "Deny" &&
      jsondecode(aws_iam_role_policy.invoke.policy).Statement[1].Condition.StringNotEquals["aws:RequestedRegion"] ==
    ["ap-southeast-2", "ap-southeast-4"])
    error_message = "an explicit deny refuses every region outside Australia"
  }
}

run "a_region_outside_australia_is_refused" {
  # Kills a regions variable that accepts anything.
  command = plan
  variables {
    regions = ["ap-southeast-2", "us-east-1"]
  }
  expect_failures = [var.regions]
}

run "the_account_id_is_twelve_digits" {
  command = plan
  variables {
    aws_account_id = "12345"
  }
  expect_failures = [var.aws_account_id]
}

run "the_issuer_is_https" {
  # Kills an http:// issuer, which STS would never trust and which hides a typo.
  command = plan
  variables {
    oidc_issuer_url = "http://example.invalid/"
  }
  expect_failures = [var.oidc_issuer_url]
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd infra/terraform/modules/bedrock-federation && terraform init -backend=false -input=false && terraform test'
```

Expected: `terraform init` fails with `Terraform encountered problems while reading configuration` or, once only the test exists, `terraform test` fails with `Reference to undeclared resource` for `aws_iam_role.this`. `terraform init` downloads the AWS provider from `registry.terraform.io`; that the corporate network allows it is **UNVERIFIED** (phase 8 depends on the same registry for `azurerm`).

- [ ] **Step 3: Write `versions.tf` and `variables.tf`**

`versions.tf`. Terraform's range is phase 8's (`task-03.md:32`); `mock_provider` needs 1.7 or later, which the range already implies. A module takes a provider range and the calling root pins the exact version in its lock file:

```hcl
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    aws = { source = "hashicorp/aws", version = ">= 5.70.0, < 7.0.0" }
  }
}
```

`variables.tf`:

```hcl
variable "oidc_issuer_url" {
  type        = string
  description = "The AKS cluster's OIDC issuer: phase 8 foundation output aks_oidc_issuer_url."
  validation {
    condition     = startswith(var.oidc_issuer_url, "https://")
    error_message = "oidc_issuer_url must be an https:// URL."
  }
}

variable "aws_account_id" {
  type        = string
  description = "The AWS account that holds Bedrock for the AU route (owner question Q3)."
  validation {
    condition     = can(regex("^[0-9]{12}$", var.aws_account_id))
    error_message = "aws_account_id must be twelve digits."
  }
}

variable "namespace" {
  type    = string
  default = "panacea-gateway"
}

variable "service_account" {
  type    = string
  default = "litellm"
}

variable "regions" {
  type        = list(string)
  description = "Where the role may invoke. Australia only: Sydney, and Melbourne for the au. profiles."
  default     = ["ap-southeast-2", "ap-southeast-4"]
  validation {
    condition = length(var.regions) > 0 && alltrue([
    for r in var.regions : contains(["ap-southeast-2", "ap-southeast-4"], r)])
    error_message = "regions may name only ap-southeast-2 and ap-southeast-4."
  }
}

variable "role_name" {
  type    = string
  default = "panacea-gateway-bedrock-au"
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

- [ ] **Step 4: Write `main.tf` and `outputs.tf`**

`main.tf`. No `thumbprint_list`: in current AWS provider versions it is optional and AWS verifies well-known issuers through its own trust store (training knowledge, **UNVERIFIED**); if the pinned provider requires it, the thumbprint of the issuer's root certificate is added here and nowhere else:

```hcl
locals {
  issuer  = trimsuffix(trimprefix(var.oidc_issuer_url, "https://"), "/")
  subject = "system:serviceaccount:${var.namespace}:${var.service_account}"
}

resource "aws_iam_openid_connect_provider" "this" {
  url            = var.oidc_issuer_url
  client_id_list = ["sts.amazonaws.com"]
  tags           = var.tags
}

resource "aws_iam_role" "this" {
  name                 = var.role_name
  max_session_duration = 3600
  tags                 = var.tags
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "LiteLLMServiceAccountOnly"
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.this.arn }
      Condition = { StringEquals = {
        "${local.issuer}:sub" = local.subject
        "${local.issuer}:aud" = "sts.amazonaws.com"
      } }
    }]
  })
}

resource "aws_iam_role_policy" "invoke" {
  name = "invoke-au-profiles-only"
  role = aws_iam_role.this.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "InvokeAuProfilesOnly"
        Effect = "Allow"
        Action = ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"]
        Resource = concat(
          [for r in var.regions : "arn:aws:bedrock:${r}:${var.aws_account_id}:inference-profile/au.anthropic.*"],
          [for r in var.regions : "arn:aws:bedrock:${r}::foundation-model/anthropic.*"],
        )
      },
      {
        Sid       = "DenyOutsideAustralia"
        Effect    = "Deny"
        Action    = "*"
        Resource  = "*"
        Condition = { StringNotEquals = { "aws:RequestedRegion" = var.regions } }
      },
    ]
  })
}
```

`outputs.tf`:

```hcl
output "role_arn" {
  description = "Task 19's bedrock.roleArn: LiteLLM's AWS_ROLE_ARN."
  value       = aws_iam_role.this.arn
}

output "oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.this.arn
}

output "allowed_regions" {
  value = var.regions
}
```

- [ ] **Step 5: Run the tests to verify they pass, and that they can fail**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd infra/terraform/modules/bedrock-federation && terraform init -backend=false -input=false && terraform fmt -check -recursive && terraform validate && terraform test'
```

Expected: `Success! The configuration is valid.`, then five runs passing and `Success! 5 passed, 0 failed.` The `.terraform/` directory `init` creates is already ignored (`.gitignore:7`); the module commits no lock file, since the calling root owns the pin.

**Prove three checks are not vacuous**, restoring each change after:
1. Change the trust's `StringEquals` to `StringLike` and the subject to `system:serviceaccount:panacea-gateway:*`: `trust_is_exactly_one_service_account` must fail on two asserts.
2. Add `"arn:aws:bedrock:ap-southeast-2:${var.aws_account_id}:inference-profile/apac.anthropic.*"` to `Resource`: `invoke_stays_in_australia` must fail with `every resource is an au.anthropic profile…`.
3. Delete the `DenyOutsideAustralia` statement: the same run must fail on its deny assert (an index error on `Statement[1]` counts as a fail).

- [ ] **Step 6: Commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'git add infra/terraform/modules/bedrock-federation/versions.tf infra/terraform/modules/bedrock-federation/variables.tf infra/terraform/modules/bedrock-federation/main.tf infra/terraform/modules/bedrock-federation/outputs.tf infra/terraform/modules/bedrock-federation/tests/federation.tftest.hcl'
~/.claude/tools/claudezsh/claudezsh 'git commit -F /tmp/msg-task20.txt -- infra/terraform/modules/bedrock-federation'
```

`/tmp/msg-task20.txt`:

```text
feat(infra): bedrock-federation, the AU route with no AWS key

An IAM OIDC provider for the AKS cluster's issuer, one role that only
panacea-gateway/litellm may assume (StringEquals on sub and aud), and an
inline policy that may invoke Anthropic models through au. inference
profiles in Sydney and Melbourne and nothing else, with an explicit deny
on every other region. Policies are jsonencode'd so mock-provider tests
can read them.

Written and tested with mock_provider only; never applied. The account is
owner question Q3, and the calling stack is phase 8's, fed by its
aks_oidc_issuer_url output.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**What the calling stack must do, for phase 8** (stated so that ask P8-5 has a concrete shape, not built here): configure the `aws` provider for `ap-southeast-2` with the owner's account (Q3) and its own OIDC-federated pipeline identity, never an access key; pass `aws_account_id` and `oidc_issuer_url = <foundation>.aks_oidc_issuer_url`; hand `role_arn` to the gateway chart as `bedrock.roleArn`; and switch off the account's Bedrock model-invocation logging, or keep it off, which task 24's P05 checks.

**What this task does NOT claim:**

- **An AWS account, an apply, or a real exchange.** Nothing here has spoken to AWS; the first `AssumeRoleWithWebIdentity` is task 24's P04.
- **That the `au.` profiles exist for the models task 02 lists, or that they stay in Sydney and Melbourne.** Both are from search summaries (`05-design.md:225-227`), **UNVERIFIED**; the region deny is what makes a wrong belief fail closed.
- **The ARN shapes and the condition-key form.** The inference-profile ARN with an account id, the foundation-model ARN with none, the need for both, and IAM's naming of OIDC condition keys (issuer without scheme or trailing slash) are remembered, **UNVERIFIED**.
- **That LiteLLM uses this role.** That rests on boto3's default chain reading `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE` (task 02's decision (d), task 19's decision (b)), **UNVERIFIED** until P04.
- **Bedrock's data retention.** The module controls who may call and where; what AWS keeps is the org's agreement (Q3) and the account's invocation-logging setting (P05).
