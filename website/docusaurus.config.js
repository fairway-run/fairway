// @ts-check

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Fairway',
  tagline: 'Governed Agentic Engineering',
  favicon: 'img/logo.svg',
  url: 'https://fairway.run',
  baseUrl: '/',
  organizationName: 'fairway-run',
  projectName: 'fairway',
  trailingSlash: false,
  onBrokenLinks: 'warn',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn'
    }
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: '../docs',
          routeBasePath: 'docs',
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl: 'https://github.com/fairway-run/fairway/tree/main/docs/'
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css')
        }
      })
    ]
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/social-card.png',
      navbar: {
        title: 'Fairway',
        logo: {
          alt: 'Fairway',
          src: 'img/logo.svg'
        },
        items: [
          { to: '/docs/quickstart', label: 'Docs', position: 'left' },
          { to: '/docs/governed-agentic-engineering', label: 'Model', position: 'left' },
          { to: '/docs/agent-guide', label: 'Agent Guide', position: 'left' },
          { to: '/docs/design/dashboard', label: 'Dashboard', position: 'left' },
          { to: '/docs/config-reference', label: 'Reference', position: 'left' },
          {
            href: 'https://github.com/fairway-run/fairway',
            label: 'GitHub',
            position: 'right'
          }
        ]
      },
      footer: {
        style: 'light',
        links: [
          {
            title: 'Start',
            items: [
              { label: 'Quickstart', to: '/docs/quickstart' },
              { label: 'Product', to: '/docs/product' },
              { label: 'Governed Agentic Engineering', to: '/docs/governed-agentic-engineering' },
              { label: 'Product Boundaries', to: '/docs/design/product-boundaries' },
              { label: 'Backlog Sources', to: '/docs/design/backlog-sources' }
            ]
          },
          {
            title: 'Operate',
            items: [
              { label: 'Agent Guide', to: '/docs/agent-guide' },
              { label: 'Dashboard', to: '/docs/design/dashboard' },
              { label: 'Workstream Profiles', to: '/docs/workstream-profile-guide' },
              { label: 'Review Lanes', to: '/docs/design/review-lanes' }
            ]
          },
          {
            title: 'Project',
            items: [
              { label: 'GitHub', href: 'https://github.com/fairway-run/fairway' },
              { label: 'Release Notes', to: '/docs/release-notes' },
              { label: 'Release Process', to: '/docs/governance/release' }
            ]
          }
        ],
        copyright: `Copyright ${new Date().getFullYear()} Fairway contributors.`
      },
      colorMode: {
        defaultMode: 'light',
        respectPrefersColorScheme: true
      },
      prism: {
        additionalLanguages: ['bash', 'toml', 'yaml', 'go']
      }
    })
};

module.exports = config;
