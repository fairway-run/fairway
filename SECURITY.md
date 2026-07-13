# Fairway Security Policy

## Reporting a Vulnerability

Report suspected vulnerabilities through GitHub private vulnerability reporting:

<https://github.com/fairway-run/fairway/security/advisories/new>

Do not open a public issue with exploit details, credentials, private customer
data, signing material, or unpublished advisory content. Fairway maintainers
will acknowledge the report, establish a privacy-bounded tracking reference,
and coordinate disclosure with the reporter. This policy states engineering
targets, not a contractual service-level agreement.

## Supported Release Lines

Fairway uses three support tracks:

| Track | Eligibility | Security maintenance | End-of-support notice |
|---|---|---|---|
| `standard` | Latest patch of the current minor line | Critical and high-severity fixes until 90 days after the next minor release | Release notes and a signed advisory when the date is known |
| `lts` | A release explicitly designated LTS in signed release notes | Security fixes for 18 months from the named release date | At least 90 days before end of support when practicable |
| `emergency` | A bounded response channel, not a support duration | A reviewed urgent patch for a named affected line | The advisory names superseded artifacts and trust roots |

No release is LTS merely because it appears in an offline bundle. The release
notes and signed restricted-channel advisory must name the support track,
supported versions, fixed versions, rollback bundle, and end-of-support state.
Unsupported releases may still receive a mitigation notice, but patch
availability is not implied.

## Response Targets

Targets begin after a report is reproducible and severity is assigned:

| Severity | Triage target | Patch or reviewed mitigation target |
|---|---:|---:|
| Critical | 1 business day | 7 calendar days |
| High | 3 business days | 30 calendar days |
| Medium | 10 business days | 90 calendar days or next planned release |
| Low | 20 business days | Next planned release or documented deferral |

Exceptions require a recorded owner, reason, mitigation, next review date, and
customer-notification decision. Scanner silence is not proof that a release is
not affected. VEX status must carry a reviewed justification.

## Restricted and Disconnected Delivery

Connected publication may use GitHub Security Advisories and release notes.
Restricted environments use the signed package documented in
[Restricted Advisory and LTS Patch Channel](docs/security/restricted-advisory-channel.md).
The package binds machine-readable and human-readable advisory views to an
opaque offline patch bundle by digest and identifier. Verification requires a
separately pinned Ed25519 public key and exact advisory, patch, and rollback
identifiers.

Customer acknowledgement records `received`, `deferred`, or `rejected`. It does
not import, install, approve, or deploy a patch. The customer retains software
intake, maintenance-window, rollback, and deployment authority.

## Emergency Signing and Trust Roots

Emergency publication still requires independent release/security review,
signed immutable artifacts, exact affected/fixed versions, rollback identity,
and retained verification evidence. Signing keys come from the release secret
boundary and are never stored in the repository, advisory package, task text,
or evidence notes.

Planned trust-root rotation uses an overlap advisory signed by the current root
and distributes the next root through the approved software-intake channel.
If the current root is suspected compromised, do not trust an in-band rotation
signed only by that root. Publish a bounded incident notice through an
independent trusted channel, distribute the replacement root through customer
software intake, revoke the old root, and issue new advisory and patch bundle
identifiers. Existing customer import and deployment controls remain in force.

## Nonclaims

Fairway security advisories and evidence packages do not establish external
certification, compliance, authorization, approval, or risk acceptance. A
successful signature and digest check proves package integrity and pinned-key
identity only. It does not prove absence of vulnerabilities or authorize live
operations.
