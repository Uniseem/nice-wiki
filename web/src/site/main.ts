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

startTheme()
startDrawers()
startTree()
startTocSpy()
startCodeCopy()
startCopyMarkdown()
startBanner()
startHeroVideo()
startCitationHighlight()
document.documentElement.dataset.ready = 'true'
