// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    {
      type: 'category',
      label: 'Evaluate',
      items: [
        'product',
        'case-studies/ai-cloud',
        'design/product-boundaries',
        'architecture',
        'release-highlights'
      ]
    },
    {
      type: 'category',
      label: 'Get Started',
      items: [
        'quickstart',
        'quality-record-demo',
        'design/concepts'
      ]
    },
    {
      type: 'category',
      label: 'Use Fairway',
      items: [
        'agent-guide',
        'design/dashboard',
        'design/quality-workspace',
        'design/common-path-automation',
        'design/task-decision-memory',
        'design/work-batch-model',
        'design/context-packets',
        'design/checkpoints',
        'design/review-policy-profiles',
        'design/review-lanes',
        'design/review-wait-notification-model',
        'design/session-launch',
        {
          type: 'category',
          label: 'Assurance evidence',
          items: [
            'design/assurance-profiles',
            'assurance/starter-profiles',
            'assurance/authoring',
            'assurance/compatibility',
            'design/assurance-packages',
            'security/sovereign-deployment-ready',
            'security/sovereign-threat-model',
            'security/sovereign-security-target-draft',
            'security/sovereign-data-inventory',
            'assessment/fairway-sovereign-assessor-package-v1',
            'security/sovereign-identity-authorization',
            'security/sovereign-cryptography-key-posture',
            'security/sovereign-audit-integrity',
            'security/release-assurance-bundle',
            'security/restricted-advisory-channel',
            'operations/sovereign-offline-bundle'
          ]
        }
      ]
    },
    {
      type: 'category',
      label: 'Operate',
      items: [
        'design/reports',
        'design/environment-deploy-preflight',
        'design/live-operation-control-room',
        'operations/small-team-lab-deployment',
        'operations/sovereign-key-readiness',
        'operations/sovereign-audit-export',
        'operations/sovereign-deployment-baselines',
        'design/shared-team-deployment-operations',
        'design/dashboard-sharing',
        'release-notes',
        'governance/release'
      ]
    },
    {
      type: 'category',
      label: 'Integrate',
      items: [
        'ecosystem',
        'integrations',
        'workstream-profile-guide',
        'design/provider-surface-capability-readiness',
        'design/provider-notifications',
        'design/issue-tracker-integrations',
        'design/rule-packs',
        'design/postgres-adapter',
        'design/trusted-proxy-identity-verification'
      ]
    },
    {
      type: 'category',
      label: 'Understand',
      items: [
        'governed-agentic-engineering',
        'design/agent-native-product-interface',
        'design/coordination-intelligence',
        'design/small-team-autonomy-operating-model',
        'design/shared-team-operating-model',
        'design/supply-chain-provenance',
        'design/delivery-resources'
      ]
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'config-reference',
        'design/cli',
        'design/schema',
        'design/state-machine',
        'design/hierarchy',
        'design/worktrees',
        'design/multi-project',
        'design/backlog-sources',
        'design/workstream-profiles',
        'design/regression-packets',
        'design/watchers',
        'design/provider-usage-accounting',
        'docs-portal',
        {
          type: 'category',
          label: 'Governance',
          items: [
            'governance/README',
            'governance/coding-standards',
            'governance/testing',
            'governance/review-guards',
            'governance/commits'
          ]
        }
      ]
    },
    {
      type: 'category',
      label: 'Archive',
      collapsed: true,
      items: ['archive/README']
    }
  ]
};

module.exports = sidebars;
