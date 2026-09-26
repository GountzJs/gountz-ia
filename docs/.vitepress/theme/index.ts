import { h } from 'vue'
import DefaultTheme from 'vitepress/theme'
import './style.css'
import MermaidZoom from './MermaidZoom.vue'

export default {
  extends: DefaultTheme,
  Layout() {
    return h(DefaultTheme.Layout, null, {
      'doc-after': () => h(MermaidZoom)
    })
  }
}
