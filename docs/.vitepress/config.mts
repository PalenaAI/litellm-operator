import { defineConfig } from 'vitepress'
import { readFileSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))

function getLatestVersion(): string {
  const changelog = readFileSync(resolve(__dirname, '../changelog.md'), 'utf-8')
  const match = changelog.match(/^## \[(\d+\.\d+\.\d+)\]/m)
  return match ? `v${match[1]}` : 'latest'
}

export default defineConfig({
  title: 'LiteLLM Operator',
  description: 'Kubernetes operator for deploying and managing production-ready LiteLLM AI Gateway instances',
  ignoreDeadLinks: [
    /\/LICENSE$/,
  ],

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.png' }],
  ],

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Reference', link: '/reference/crds' },
      {
        text: getLatestVersion(),
        items: [
          { text: 'Changelog', link: '/changelog' },
          { text: 'Contributing', link: '/contributing' },
        ],
      },
    ],

    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'What is LiteLLM Operator?', link: '/guide/what-is-litellm-operator' },
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'Installation', link: '/guide/installation' },
        ],
      },
      {
        text: 'Core Concepts',
        items: [
          { text: 'Architecture', link: '/guide/architecture' },
          { text: 'Config Sync', link: '/guide/config-sync' },
          { text: 'Team Member Management', link: '/guide/team-members' },
          { text: 'Virtual Key Secrets', link: '/guide/virtual-keys' },
        ],
      },
      {
        text: 'Configuration',
        items: [
          { text: 'Admin UI', link: '/guide/admin-ui' },
          { text: 'Caching', link: '/guide/caching' },
          { text: 'Enterprise License', link: '/guide/enterprise-license' },
          { text: 'SSO Setup', link: '/guide/sso' },
          { text: 'JWT / OAuth2 Auth', link: '/guide/jwt-oauth2-auth' },
          { text: 'RBAC', link: '/guide/rbac' },
          { text: 'SCIM Provisioning', link: '/guide/scim' },
          { text: 'Secret Managers', link: '/guide/secret-managers' },
          { text: 'Database', link: '/guide/database' },
          { text: 'Observability', link: '/guide/observability' },
        ],
      },
      {
        text: 'CRD Reference',
        items: [
          { text: 'Overview', link: '/reference/crds' },
          { text: 'LiteLLMInstance', link: '/reference/litellminstance' },
          { text: 'LiteLLMOrganization', link: '/reference/litellmorganization' },
          { text: 'LiteLLMModel', link: '/reference/litellmmodel' },
          { text: 'LiteLLMTeam', link: '/reference/litellmteam' },
          { text: 'LiteLLMUser', link: '/reference/litellmuser' },
          { text: 'LiteLLMCustomer', link: '/reference/litellmcustomer' },
          { text: 'LiteLLMCredential', link: '/reference/litellmcredential' },
          { text: 'LiteLLMGuardrail', link: '/reference/litellmguardrail' },
          { text: 'LiteLLMVirtualKey', link: '/reference/litellmvirtualkey' },
        ],
      },
      {
        text: 'API Client',
        items: [
          { text: 'LiteLLM API Client', link: '/reference/api-client' },
        ],
      },
      {
        text: 'Development',
        items: [
          { text: 'Contributing', link: '/contributing' },
          { text: 'Testing', link: '/reference/testing' },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/PalenaAI/litellm-operator' },
    ],

    search: {
      provider: 'local',
    },

    editLink: {
      pattern: 'https://github.com/PalenaAI/litellm-operator/edit/main/docs/:path',
    },

    footer: {
      message: 'Released under the Apache 2.0 License.',
      copyright: 'Copyright 2026 bitkaio LLC',
    },
  },
})
