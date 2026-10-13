import './site.css'
import { startTheme } from './enhance/theme'
import { startDrawers } from './enhance/drawer'
import { startTree } from './enhance/tree'
import { startTocSpy } from './enhance/toc-spy'
import { startCodeCopy } from './enhance/code-copy'
import { startCopyMarkdown } from './enhance/copy-markdown'
import { startBanner } from './enhance/banner'
import { startHeroVideo } from './enhance/hero-video'
import { startCitationHighlight } from './enhance/citation-highlight'
import { startMath } from './enhance/math'
import { startVideoFacades } from './enhance/video-facade'
import { startBeacon } from './enhance/beacon'

startTheme()
startDrawers()
startTree()
startTocSpy()
startCodeCopy()
startCopyMarkdown()
startBanner()
startHeroVideo()
startCitationHighlight()
startMath()
startVideoFacades()
startBeacon()
document.querySelectorAll<HTMLSelectElement>('[data-language-switch]').forEach(select => {
  select.value = document.documentElement.lang
  select.addEventListener('change', () => {
    const parts = location.pathname.split('/')
    if (parts.length > 1) parts[1] = select.value
    document.cookie = `nw_language=${encodeURIComponent(select.value)}; Path=/; Max-Age=31536000; SameSite=Lax${location.protocol === 'https:' ? '; Secure' : ''}`
    location.assign(`${parts.join('/')}${location.search}${location.hash}`)
  })
})
document.documentElement.dataset.ready = 'true'

document.querySelectorAll<HTMLElement>('[data-island="ask-widget"],[data-island="ask-page"]').forEach(async element => {
  const { createApp } = await import('vue')
  const component = element.dataset.island === 'ask-page' ? (await import('./islands/AskPage.vue')).default : (await import('./islands/AskWidget.vue')).default
  createApp(component, { page: element.dataset.page, turnstileSiteKey: element.dataset.turnstileSiteKey }).mount(element)
})
document.querySelectorAll<HTMLElement>('[data-island="language-suggest"]').forEach(async element => { const { createApp } = await import('vue'); const component=(await import('./islands/LanguageSuggest.vue')).default;createApp(component,{current:element.dataset.current??''}).mount(element) })
