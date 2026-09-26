import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

export default withMermaid(
  defineConfig({
  title: 'Gountz IA',
  description: 'Gountz IA (gz-ia) — Harness Universal de Terminal para Agentes de IA, Microkernel Orchy y Aislamiento en Git Worktrees',
  lang: 'es-ES',
  cleanUrls: true,
  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/logo.png' }],
    ['meta', { name: 'theme-color', content: '#38bdf8' }],
  ],
  themeConfig: {
    logo: '/logo.png',
    siteTitle: 'Gountz IA',
    nav: [
      { text: 'Guía', link: '/guide/quickstart', activeMatch: '/guide/' },
      { text: 'Arquitectura', link: '/architecture/overview', activeMatch: '/architecture/' },
      { text: 'Microkernel Orchy', link: '/orchy/microkernel', activeMatch: '/orchy/' },
      { text: 'Referencia CLI & API', link: '/reference/cli', activeMatch: '/reference/' },
      {
        text: 'v0.0.1',
        items: [
          { text: 'Changelog', link: '/reference/changelog' },
          { text: 'Repositorio', link: 'https://github.com/GountzJs/gountz-ia' }
        ]
      }
    ],
    sidebar: {
      '/guide/': [
        {
          text: 'Guía de Inicio',
          items: [
            { text: '¿Qué es gz-ia?', link: '/guide/what-is-gz-ia' },
            { text: 'Inicio Rápido (Quickstart)', link: '/guide/quickstart' },
            { text: 'Agentes Soportados (Multi-Driver)', link: '/guide/providers' },
            { text: 'Perfiles Agénticos y Skills', link: '/guide/profiles' },
            { text: 'Gestión de Secretos (Vault)', link: '/guide/vault' },
            { text: 'Uso de la TUI Interactiva', link: '/guide/tui' },
            { text: 'Flujos de Trabajo del Mundo Real', link: '/guide/workflows' }
          ]
        }
      ],
      '/architecture/': [
        {
          text: 'Arquitectura del Sistema',
          items: [
            { text: 'Visión General', link: '/architecture/overview' },
            { text: 'Contratos e Interfaces Go', link: '/architecture/contracts' },
            { text: 'Aislamiento con Git Worktrees', link: '/architecture/worktrees' },
            { text: 'Ciclo de Vida de Sesiones', link: '/architecture/sessions' },
            { text: 'Observabilidad y Logger de Eventos', link: '/architecture/observability' },
            { text: 'Métricas y Telemetría', link: '/architecture/metrics' }
          ]
        }
      ],
      '/orchy/': [
        {
          text: 'Microkernel Orchy',
          items: [
            { text: 'Conceptos y Filosofía', link: '/orchy/microkernel' },
            { text: 'Plugins y Service Container', link: '/orchy/plugins-services' },
            { text: 'Servidor MCP Nativo', link: '/orchy/mcp' },
            { text: 'Baterías Incluidas', link: '/orchy/batteries' },
            { text: 'Creación de Plugins y Herramientas', link: '/orchy/custom-plugin' }
          ]
        }
      ],
      '/reference/': [
        {
          text: 'Referencia y Protocolos',
          items: [
            { text: 'Referencia Completa del CLI', link: '/reference/cli' },
            { text: 'Códigos de Salida y Errores', link: '/reference/exit-codes' },
            { text: 'Protocolo de Logger', link: '/reference/logger' },
            { text: 'Esquemas de MCP Tools', link: '/reference/mcp-tools' },
            { text: 'Variables de Entorno y Configuración', link: '/reference/env' },
            { text: 'Changelog v0.0.1', link: '/reference/changelog' }
          ]
        }
      ]
    },
    search: {
      provider: 'local',
      options: {
        locales: {
          root: {
            translations: {
              button: {
                buttonText: 'Buscar',
                buttonAriaLabel: 'Buscar'
              },
              modal: {
                noResultsText: 'No se encontraron resultados',
                resetButtonTitle: 'Limpiar búsqueda',
                footer: {
                  selectText: 'para seleccionar',
                  navigateText: 'para navegar',
                  closeText: 'para cerrar'
                }
              }
            }
          }
        }
      }
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/GountzJs/gountz-ia' }
    ],
    footer: {
      message: 'Gountz IA (gz-ia) — Universal Terminal AI Harness',
      copyright: 'Copyright © 2026 Tomas & gz-ia Contributors'
    },
    outline: {
      level: [2, 3],
      label: 'En esta página'
    },
    docFooter: {
      prev: 'Página anterior',
      next: 'Página siguiente'
    }
  },
  vite: {
    optimizeDeps: {
      include: [
        'mermaid',
        'fastdom',
        'fastdom/extensions/fastdom-promised.js',
        'debug'
      ]
    }
  }
}))

