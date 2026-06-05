// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    {
      type: 'category',
      label: 'Start Here',
      items: [
        'quickstart',
        'product',
        'design/scope',
        'design/concepts',
        'release-notes',
        'docs-portal'
      ]
    },
    {
      type: 'category',
      label: 'Operators And Agents',
      items: [
        'agent-guide',
        'design/dashboard',
        'design/coordinator-loop',
        'design/context-packets',
        'design/checkpoints',
        'design/review-lanes',
        'design/session-launch',
        'design/watchers'
      ]
    },
    {
      type: 'category',
      label: 'Configuration And Reference',
      items: [
        'config-reference',
        'workstream-profile-guide',
        'design/workstream-profiles',
        'design/cli',
        'design/schema',
        'design/state-machine',
        'design/hierarchy',
        'design/worktrees',
        'design/multi-project'
      ]
    },
    {
      type: 'category',
      label: 'Governance',
      items: [
        'governance/README',
        'governance/coding-standards',
        'governance/testing',
        'governance/review-guards',
        'governance/commits',
        'governance/release'
      ]
    },
    {
      type: 'category',
      label: 'Design And Roadmap',
      items: [
        'design/release-cuts',
        'design/implementation-roadmap',
        'design/regression-packets',
        'design/postgres-adapter',
        'design/issue-tracker-integrations'
      ]
    },
    {
      type: 'category',
      label: 'Archive',
      items: [
        'archive/README'
      ]
    }
  ]
};

module.exports = sidebars;
