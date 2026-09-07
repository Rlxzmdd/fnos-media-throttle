// This UI intentionally keeps its template next to the small application
// controller. The compiler-enabled Vue build is therefore required in the
// production bundle; the runtime-only build silently renders an empty comment.
import { computed, createApp, onMounted, onUnmounted, reactive, ref, watch } from 'vue/dist/vue.esm-bundler.js'
import './style.css'
import SettingsPanel from './components/SettingsPanel.js'
import ChainLimits from './components/ChainLimits.js'
import { formatLogMessage } from './log-format.js'

import { api } from './api.js'
import { blankChain, blankLibrary, blankDownloader } from './forms.js'
import { icons } from './icons.js'
import { applyTheme, followFNOSTheme, loadAppTheme, saveAppTheme } from './theme.js'

const application = createApp({
  components: { SettingsPanel, ChainLimits },
  setup () {
    const page = ref('home')
    const libraries = ref([]), downloaders = ref([]), chains = ref([]), events = ref([])
    const settings = reactive({ debug: false, releaseMode: 'original', appVersion: '' })
    const loading = ref(false), busy = ref(false), message = ref(''), error = ref('')
    const theme = ref(loadAppTheme(localStorage))
    const modal = reactive({ library: false, downloader: false, chain: false, confirm: false })
    const libraryForm = reactive(blankLibrary()), downloaderForm = reactive(blankDownloader()), chainForm = reactive(blankChain())
    const confirmation = reactive({ title: '', text: '', run: null })
    const eventPage = ref(1), eventSize = 15
    const nav = [
      { id: 'home', label: '首页', icon: icons.home }, { id: 'libraries', label: '影视库', icon: icons.media },
      { id: 'downloaders', label: '下载器', icon: icons.download }, { id: 'chains', label: '绑定链', icon: icons.chain },
      { id: 'logs', label: '日志', icon: icons.logs }
    ]
    const titles = { home: ['首页', '观看状态与绑定链概览'], libraries: ['影视库', '每个影视库独立采集观看人数。'], downloaders: ['下载器', '下载器仅保存连接与运行状态；限速规则由绑定链管理。'], chains: ['绑定链', '多影视库 × 多下载器，共享一套独立限速规则。'], logs: ['运行日志', '连接、采集与限速变更记录'], settings: ['设置', '外观与应用信息'] }
    const activeViewers = computed(() => libraries.value.filter(x => x.enabled).reduce((sum, x) => sum + (x.lastCount || 0), 0))
    const healthyDownloaders = computed(() => downloaders.value.filter(x => x.enabled && !x.lastError).length)
    const enabledChains = computed(() => chains.value.filter(x => x.enabled).length)
    const pagedEvents = computed(() => events.value.slice((eventPage.value - 1) * eventSize, eventPage.value * eventSize))
    const eventPageCount = computed(() => Math.max(1, Math.ceil(events.value.length / eventSize)))
    const libraryName = id => libraries.value.find(x => x.id === id)?.name || `影视库 #${id}`
    const downloaderName = id => downloaders.value.find(x => x.id === id)?.name || `下载器 #${id}`
    const kindName = kind => ({ fnos: '飞牛影视', emby: 'Emby', jellyfin: 'Jellyfin', http: 'HTTP 人数接口' }[kind] || kind)
    const downloaderKindName = kind => ({ fnos: '飞牛下载', qbittorrent: 'qBittorrent', transmission: 'Transmission' }[kind] || kind)
    const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '尚未同步'

    let stopFollowingTheme = () => {}
    watch(theme, () => { saveAppTheme(localStorage, theme.value); applyTheme(theme.value) })

    const refresh = async () => {
      loading.value = true
      try {
        const [l, d, c, e, s] = await Promise.all([api('/libraries'), api('/downloaders'), api('/chains'), api('/events'), api('/settings')])
        libraries.value = l; downloaders.value = d; chains.value = c; events.value = e; Object.assign(settings, s)
        eventPage.value = Math.min(eventPage.value, eventPageCount.value)
      } catch (err) { error.value = err.message } finally { loading.value = false }
    }
    const run = async (action, success = '') => {
      busy.value = true; error.value = ''; message.value = ''
      try { await action(); if (success) message.value = success; await refresh() } catch (err) { error.value = err.message } finally { busy.value = false }
    }
    const reset = (target, value) => Object.assign(target, value)
    const go = target => { page.value = target; error.value = ''; message.value = '' }
    const setDownloaderKind = () => { if (downloaderForm.kind === 'fnos' && !downloaderForm.baseUrl) downloaderForm.baseUrl = 'ws://127.0.0.1:5666/websocket?type=main' }
    const openLibrary = item => { reset(libraryForm, item ? { ...blankLibrary(), ...item, apiKey: '' } : blankLibrary()); modal.library = true }
    const openDownloader = item => { reset(downloaderForm, item ? { ...blankDownloader(), ...item, password: '' } : blankDownloader()); modal.downloader = true }
    const openChain = item => { reset(chainForm, item ? JSON.parse(JSON.stringify(item)) : blankChain()); modal.chain = true }
    const saveLibrary = () => run(async () => { const method = libraryForm.id ? 'PUT' : 'POST'; const path = libraryForm.id ? `/libraries/${libraryForm.id}` : '/libraries'; await api(path, { method, body: JSON.stringify(libraryForm) }); modal.library = false }, '影视库已保存')
    const saveDownloader = () => run(async () => { const method = downloaderForm.id ? 'PUT' : 'POST'; const path = downloaderForm.id ? `/downloaders/${downloaderForm.id}` : '/downloaders'; await api(path, { method, body: JSON.stringify(downloaderForm) }); modal.downloader = false }, '下载器已保存')
    const saveChain = () => run(async () => { const method = chainForm.id ? 'PUT' : 'POST'; const path = chainForm.id ? `/chains/${chainForm.id}` : '/chains'; await api(path, { method, body: JSON.stringify(chainForm) }); modal.chain = false }, '绑定链已保存')
    const test = (kind, id) => run(() => api(`/${kind}/${id}/test`, { method: 'POST' }), '连接正常')
    const restore = id => run(() => api(`/downloaders/${id}/restore`, { method: 'POST' }), '已恢复原始限速')
    const askDelete = (kind, item) => { confirmation.title = `删除${kind === 'chains' ? '绑定链' : kind === 'libraries' ? '影视库' : '下载器'}`; confirmation.text = `确定删除“${item.name}”吗？`; confirmation.run = () => run(async () => { await api(`/${kind}/${item.id}`, { method: 'DELETE' }); modal.confirm = false }, '已删除'); modal.confirm = true }
    const toggleChainMember = (list, id) => { const index = list.indexOf(id); if (index >= 0) list.splice(index, 1); else list.push(id) }
    const saveSettings = changes => run(async () => {
      const saved = await api('/settings', { method: 'PUT', body: JSON.stringify({ ...settings, ...changes }) })
      Object.assign(settings, saved)
    }, '设置已保存')
    const exportConfig = async () => {
      busy.value = true; error.value = ''; message.value = ''
      try {
        const response = await fetch('/api/v1/config/export')
        if (!response.ok) {
          const body = await response.json().catch(() => ({}))
          throw new Error(body.error || `导出失败：${response.status}`)
        }
        const blob = await response.blob()
        const disposition = response.headers.get('Content-Disposition') || ''
        const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] || `fnos-media-throttle-config-${new Date().toISOString().slice(0, 10)}.json`
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url; link.download = filename; link.click()
        URL.revokeObjectURL(url)
        message.value = '配置已导出，请妥善保管备份文件'
      } catch (err) { error.value = err.message } finally { busy.value = false }
    }
    const importConfig = async file => {
      if (file.size > 10 * 1024 * 1024) { error.value = '配置文件不能超过 10 MB'; return }
      if (!window.confirm('导入会替换当前全部配置，保留现有运行日志，并先解除现有限速。是否继续？')) return
      await run(async () => {
        let backup
        try { backup = JSON.parse(await file.text()) } catch { throw new Error('配置文件不是有效的 JSON') }
        await api('/config/import', { method: 'POST', body: JSON.stringify(backup) })
      }, '配置已导入')
    }
    const pageNumbers = computed(() => Array.from({ length: eventPageCount.value }, (_, i) => i + 1).slice(Math.max(0, eventPage.value - 3), eventPage.value + 2))

    onMounted(() => { stopFollowingTheme = followFNOSTheme(() => theme.value); refresh(); setInterval(refresh, 5000) })
    onUnmounted(() => stopFollowingTheme())
    return { page, nav, titles, icons, libraries, downloaders, chains, events, settings, loading, busy, message, error, theme, modal, libraryForm, downloaderForm, chainForm, confirmation, activeViewers, healthyDownloaders, enabledChains, pagedEvents, eventPage, eventSize, eventPageCount, pageNumbers, libraryName, downloaderName, kindName, downloaderKindName, formatTime, formatLogMessage, refresh, go, setDownloaderKind, openLibrary, openDownloader, openChain, saveLibrary, saveDownloader, saveChain, test, restore, askDelete, toggleChainMember, saveSettings, exportConfig, importConfig }
  },
  template: `
  <main class="app-shell">
    <aside class="sidebar">
      <nav><button v-for="item in nav" :key="item.id" :class="{active:page===item.id}" @click="go(item.id)"><span class="nav-icon" v-html="item.icon"></span><span>{{item.label}}</span></button></nav>
      <div class="sidebar-bottom"><button :class="{active:page==='settings'}" @click="go('settings')"><span class="nav-icon" v-html="icons.settings"></span><span>设置</span></button></div>
    </aside>
    <section class="workspace"><header v-if="page!=='home'" class="topbar"><div><h1>{{titles[page][0]}}</h1><p>{{titles[page][1]}}</p></div><div class="topbar-actions"><button v-if="page==='libraries'" class="primary small" @click="openLibrary()"><span v-html="icons.plus"></span>添加影视库</button><button v-else-if="page==='downloaders'" class="primary small" @click="openDownloader()"><span v-html="icons.plus"></span>添加下载器</button><button v-else-if="page==='chains'" class="primary small" @click="openChain()"><span v-html="icons.plus"></span>新建绑定链</button><button class="icon-button" :disabled="busy" @click="refresh" title="刷新"><span v-html="icons.refresh"></span></button></div></header>
      <div class="content-scroll"><div v-if="message" class="notice success">{{message}}</div><div v-if="error" class="notice error">{{error}}<button @click="error=''">×</button></div>
        <template v-if="page==='home'"><section class="hero"><div><span class="eyebrow">首页 · 实时联动</span><h2>观影优先，下载自动让路</h2><p>绑定链汇总观看人数，按独立规则限制下载器带宽。</p></div><button class="hero-refresh" :disabled="busy" @click="refresh" title="刷新"><span v-html="icons.refresh"></span></button></section>
          <section class="stats-grid"><article><span>当前观看</span><strong>{{activeViewers}} <small>人</small></strong><em class="blue">实时汇总</em></article><article><span>影视库</span><strong>{{libraries.length}} <small>个</small></strong><em>{{libraries.filter(x=>x.enabled).length}} 个已启用</em></article><article><span>下载器</span><strong>{{downloaders.length}} <small>个</small></strong><em>{{healthyDownloaders}} 个运行正常</em></article><article><span>绑定链</span><strong>{{chains.length}} <small>条</small></strong><em>{{enabledChains}} 条已启用</em></article></section>
          <section class="panel binding-panel"><div class="section-heading"><div><h2>绑定链</h2><p>每条链拥有自己的影视库、下载器与限速规则。</p></div><button class="primary small" @click="openChain()"><span v-html="icons.plus"></span>新建绑定链</button></div><div v-if="chains.length" class="binding-list"><div v-for="chain in chains" :key="chain.id" class="binding-row chain-row" role="button" tabindex="0" @click="openChain(chain)" @keydown.enter="openChain(chain)"><div class="node media"><span v-html="icons.media"></span><div><small>{{chain.libraryIds.length}} 个影视库</small><strong>{{chain.name}}</strong></div></div><div class="flow"><i></i><span v-html="icons.chain"></span><i></i></div><div class="node downloader"><span v-html="icons.download"></span><div><small>{{chain.downloaderIds.length}} 个下载器 · {{chain.viewerCount}} 人</small><strong>{{chain.enabled?'已启用':'已停用'}}</strong></div></div></div></div><div v-else class="empty-inline"><span v-html="icons.chain"></span><div><strong>尚未建立绑定链</strong><p>添加影视库和下载器后，在这里建立联动。</p></div></div></section>
          <section class="home-columns"><section class="panel resource-panel" role="button" tabindex="0" @click="go('libraries')" @keydown.enter="go('libraries')"><div class="section-heading"><div><h2>影视库</h2><p>观看人数数据源</p></div><button class="round-add" @click.stop="openLibrary()" v-html="icons.plus"></button></div><div class="compact-list"><button v-for="item in libraries.slice(0,3)" :key="item.id" @click.stop="openLibrary(item)"><span class="resource-icon purple" v-html="icons.media"></span><div><strong>{{item.name}}</strong><small>{{kindName(item.kind)}} · {{item.lastCount}} 人观看</small></div><i :class="['status-dot',item.lastError?'warn':'ok']"></i></button><div v-if="!libraries.length" class="mini-empty">点击卡片管理影视库，或使用右上角 + 添加</div></div><button class="panel-footer" @click.stop="go('libraries')">管理所有影视库 <span>›</span></button></section>
          <section class="panel resource-panel" role="button" tabindex="0" @click="go('downloaders')" @keydown.enter="go('downloaders')"><div class="section-heading"><div><h2>下载器</h2><p>带宽控制目标</p></div><button class="round-add" @click.stop="openDownloader()" v-html="icons.plus"></button></div><div class="compact-list"><button v-for="item in downloaders.slice(0,3)" :key="item.id" @click.stop="openDownloader(item)"><span class="resource-icon blue" v-html="icons.download"></span><div><strong>{{item.name}}</strong><small>{{downloaderKindName(item.kind)}} · 当前 ↑{{item.currentUploadSpeedKB}} kb/s ↓{{item.currentDownloadSpeedKB}} kb/s</small></div><i :class="['status-dot',item.lastError?'warn':'ok']"></i></button><div v-if="!downloaders.length" class="mini-empty">点击卡片管理下载器，或使用右上角 + 添加</div></div><button class="panel-footer" @click.stop="go('downloaders')">管理所有下载器 <span>›</span></button></section></section>
        </template>
        <template v-else-if="page==='libraries'"><div class="page-actions"><div><h2>影视库</h2><p>每个影视库独立采集观看人数。</p></div><button class="primary" @click="openLibrary()"><span v-html="icons.plus"></span>添加影视库</button></div><section class="detail-grid"><article v-for="item in libraries" :key="item.id" class="detail-card"><div class="detail-head"><span class="resource-icon purple" v-html="icons.media"></span><div><h3>{{item.name}}</h3><p>{{kindName(item.kind)}}</p></div><span :class="['pill',item.enabled?'enabled':'disabled']">{{item.enabled?'已启用':'已停用'}}</span></div><dl><div><dt>活跃观看</dt><dd class="big-number">{{item.lastCount}} 人</dd></div><div><dt>轮询周期</dt><dd>{{item.pollSeconds}} 秒</dd></div><div><dt>统计本地</dt><dd>{{item.countLocalClients?'是':'否'}}</dd></div><div><dt>最近采集</dt><dd>{{formatTime(item.lastCheckedAt)}}</dd></div><div><dt>运行状态</dt><dd :class="item.lastError?'bad':'good'">{{item.lastError||'运行正常'}}</dd></div></dl><div class="card-actions"><button @click="test('libraries',item.id)">测试连接</button><button @click="openLibrary(item)">编辑</button><button @click="askDelete('libraries',item)">删除</button></div></article><button class="add-empty-card" @click="openLibrary()"><span v-html="icons.plus"></span><strong>添加影视库</strong><small>Emby、Jellyfin、飞牛影视或 HTTP 接口</small></button></section></template>
        <template v-else-if="page==='downloaders'"><div class="page-actions"><div><h2>下载器</h2><p>下载器仅保存连接与运行状态；限速规则由绑定链管理。</p></div><button class="primary" @click="openDownloader()"><span v-html="icons.plus"></span>添加下载器</button></div><section class="detail-grid"><article v-for="item in downloaders" :key="item.id" class="detail-card"><div class="detail-head"><span class="resource-icon blue" v-html="icons.download"></span><div><h3>{{item.name}}</h3><p>{{downloaderKindName(item.kind)}}</p></div><span :class="['pill',item.enabled?'enabled':'disabled']">{{item.enabled?'已启用':'已停用'}}</span></div><div class="speed-pair"><div><small>上传</small><p>当前 <strong>{{item.currentUploadSpeedKB}}</strong> kb/s</p><p>最大 <strong>{{item.currentUploadKB}}</strong> kb/s</p></div><div><small>下载</small><p>当前 <strong>{{item.currentDownloadSpeedKB}}</strong> kb/s</p><p>最大 <strong>{{item.currentDownloadKB}}</strong> kb/s</p></div></div><dl><div><dt>合计观看</dt><dd>{{item.viewerCount}} 人</dd></div><div><dt>轮询周期</dt><dd>{{item.pollSeconds}} 秒</dd></div><div><dt>最近同步</dt><dd>{{formatTime(item.lastAppliedAt)}}</dd></div><div><dt>运行状态</dt><dd :class="item.lastError?'bad':'good'">{{item.lastError||'运行正常'}}</dd></div></dl><div class="card-actions"><button @click="test('downloaders',item.id)">测试连接</button><button @click="openDownloader(item)">编辑</button><button @click="restore(item.id)">恢复原值</button><button class="danger-link" @click="askDelete('downloaders',item)">删除</button></div></article><button class="add-empty-card" @click="openDownloader()"><span v-html="icons.plus"></span><strong>添加下载器</strong><small>飞牛下载、qBittorrent 或 Transmission</small></button></section></template>
        <template v-else-if="page==='chains'"><div class="page-actions"><div><h2>绑定链</h2><p>多影视库 × 多下载器，共享一套独立限速规则。</p></div><button class="primary" @click="openChain()"><span v-html="icons.plus"></span>新建绑定链</button></div><section class="detail-grid"><article v-for="chain in chains" :key="chain.id" class="detail-card"><div class="detail-head"><span class="resource-icon blue" v-html="icons.chain"></span><div><h3>{{chain.name}}</h3><p>{{chain.libraryIds.length}} 个影视库 → {{chain.downloaderIds.length}} 个下载器</p></div><span :class="['pill',chain.enabled?'enabled':'disabled']">{{chain.enabled?'已启用':'已停用'}}</span></div><ChainLimits :chain="chain"/><dl><div><dt>合计观看</dt><dd class="big-number">{{chain.viewerCount}} 人</dd></div><div><dt>状态</dt><dd :class="chain.lastError?'bad':'good'">{{chain.lastError||'运行正常'}}</dd></div></dl><div class="card-actions"><button @click="openChain(chain)">编辑规则</button><button class="danger-link" @click="askDelete('chains',chain)">删除</button></div></article><button class="add-empty-card" @click="openChain()"><span v-html="icons.plus"></span><strong>建立绑定链</strong><small>选择影视库、下载器与限制方向</small></button></section></template>
        <template v-else-if="page==='logs'"><section class="panel log-panel"><div class="section-heading"><div><h2>最近事件</h2><p>保留连接失败、采集和限速变更记录。</p></div><button class="subtle" @click="refresh">刷新</button></div><div class="event-list"><div v-for="item in pagedEvents" :key="item.id" class="event-row"><i :class="item.level"></i><div><strong>{{item.level==='error'?'错误':item.level==='warning'?'告警':item.level==='debug'?'调试':'事件'}}</strong><p>{{formatLogMessage(item.message)}}</p></div><time>{{formatTime(item.createdAt)}}</time></div><div v-if="!events.length" class="large-empty">暂无事件</div></div><div v-if="events.length>eventSize" class="pagination"><span>共 {{events.length}} 项</span><div><button :disabled="eventPage===1" @click="eventPage--">‹</button><button v-for="number in pageNumbers" :key="number" :class="{active:eventPage===number}" @click="eventPage=number">{{number}}</button><button :disabled="eventPage===eventPageCount" @click="eventPage++">›</button></div><span>第 {{eventPage}} / {{eventPageCount}} 页</span></div></section></template>
        <SettingsPanel v-else :settings="settings" v-model:theme="theme" :busy="busy" @save="saveSettings" @export-config="exportConfig" @import-config="importConfig" />
      </div></section>
    <div v-if="modal.library" class="modal-backdrop" @mousedown.self="modal.library=false"><form class="modal" @submit.prevent="saveLibrary"><div class="modal-head"><div><h2>{{libraryForm.id?'编辑影视库':'添加影视库'}}</h2><p>连接观看人数数据源</p></div><button type="button" @click="modal.library=false" v-html="icons.close"></button></div><label>名称<input v-model.trim="libraryForm.name" placeholder="留空将按类型自动命名"></label><label>类型<select v-model="libraryForm.kind" @change="libraryForm.kind==='fnos'&&(libraryForm.baseUrl='/usr/local/apps/@appdata/trim.media/database/trimmedia.db')"><option value="fnos">飞牛影视（本地数据库）</option><option value="jellyfin">Jellyfin</option><option value="emby">Emby</option><option value="http">HTTP 人数接口</option></select></label><label>轮询周期（秒）<input v-model.number="libraryForm.pollSeconds" type="number" min="5" max="60" required></label><template v-if="libraryForm.kind==='fnos'"><label>数据库路径<input v-model.trim="libraryForm.baseUrl" required></label><label>活跃判定窗口（秒）<input v-model.number="libraryForm.activitySeconds" type="number" min="30" max="3600" required></label></template><template v-else><label>服务地址<input v-model.trim="libraryForm.baseUrl" required></label><label>API Key <small>{{libraryForm.id?'留空则保持原值':'可选'}}</small><input v-model="libraryForm.apiKey" type="password" autocomplete="new-password"></label></template><label class="check"><input v-model="libraryForm.countLocalClients" type="checkbox"><span>统计本地</span></label><p class="field-help">Emby/Jellyfin 按会话 IP 过滤；飞牛影视或仅返回总人数的 HTTP 接口缺少 IP 数据时，会显示明确提示而不会静默误算。</p><label class="check"><input v-model="libraryForm.enabled" type="checkbox"><span>启用此影视库</span></label><div class="modal-actions"><button v-if="libraryForm.id" type="button" @click="test('libraries',libraryForm.id)">测试连接</button><button type="button" @click="modal.library=false">取消</button><button class="primary" :disabled="busy">保存</button></div></form></div>
    <div v-if="modal.downloader" class="modal-backdrop" @mousedown.self="modal.downloader=false"><form class="modal wide" @submit.prevent="saveDownloader"><div class="modal-head"><div><h2>{{downloaderForm.id?'编辑下载器':'添加下载器'}}</h2><p>连接下载服务；限速规则在绑定链中设置。</p></div><button type="button" @click="modal.downloader=false" v-html="icons.close"></button></div><div class="form-grid"><label>名称<input v-model.trim="downloaderForm.name" placeholder="留空按类型命名"></label><label>类型<select v-model="downloaderForm.kind" @change="setDownloaderKind"><option value="fnos">飞牛下载（原生 WebSocket API）</option><option value="qbittorrent">qBittorrent Web API</option><option value="transmission">Transmission RPC</option></select></label><template v-if="downloaderForm.kind!=='fnos'"><label>服务地址<input v-model.trim="downloaderForm.baseUrl" required></label><label>用户名<input v-model="downloaderForm.username"></label><label>密码<input v-model="downloaderForm.password" type="password" autocomplete="new-password"></label></template><template v-else><label>网关地址<input v-model.trim="downloaderForm.baseUrl" required></label><label>飞牛用户名<input v-model.trim="downloaderForm.username" required></label><label>飞牛密码<input v-model="downloaderForm.password" :required="!downloaderForm.id" type="password"></label><label>回退用户 ID<input v-model.number="downloaderForm.userId" min="1" type="number" required></label></template><label>轮询周期（秒）<input v-model.number="downloaderForm.pollSeconds" type="number" min="5" max="60" required></label></div><label class="check"><input v-model="downloaderForm.enabled" type="checkbox"><span>启用此下载器</span></label><div class="modal-actions"><button type="button" @click="modal.downloader=false">取消</button><button class="primary" :disabled="busy">保存</button></div></form></div>
    <div v-if="modal.chain" class="modal-backdrop" @mousedown.self="modal.chain=false"><form class="modal wide chain-modal" @submit.prevent="saveChain"><div class="modal-head"><div><h2>{{chainForm.id?'编辑绑定链':'新建绑定链'}}</h2><p>一条链可以包含多个影视库与多个下载器。</p></div><button type="button" @click="modal.chain=false" v-html="icons.close"></button></div><label>名称<input v-model.trim="chainForm.name" placeholder="例如：家庭影音联动"></label><div class="binding-picker multi-binding-picker"><section class="binding-choice"><span class="picker-title"><b>1</b>影视库</span><div class="choice-list"><label v-for="item in libraries" :key="item.id" :class="['choice-row',{selected:chainForm.libraryIds.includes(item.id)}]"><input :checked="chainForm.libraryIds.includes(item.id)" type="checkbox" @change="toggleChainMember(chainForm.libraryIds,item.id)"><span><strong>{{item.name}}</strong><small>{{kindName(item.kind)}} · {{item.lastCount}} 人</small></span></label></div></section><div class="binding-direction"><i></i><span v-html="icons.chain"></span><i></i></div><section class="binding-choice"><span class="picker-title"><b>2</b>下载器</span><div class="choice-list"><label v-for="item in downloaders" :key="item.id" :class="['choice-row',{selected:chainForm.downloaderIds.includes(item.id)}]"><input :checked="chainForm.downloaderIds.includes(item.id)" type="checkbox" @change="toggleChainMember(chainForm.downloaderIds,item.id)"><span><strong>{{item.name}}</strong><small>{{downloaderKindName(item.kind)}}</small></span></label></div></section></div><section class="rule-section"><div class="rule-heading"><label class="check"><input v-model="chainForm.rule.upload.enabled" type="checkbox"><strong>限制上传</strong></label><small>启用后按观看人数限制上传带宽</small></div><div v-if="chainForm.rule.upload.enabled" class="rule-grid"><label>基础 KB/s<input v-model.number="chainForm.rule.upload.baseKB" type="number" min="0"></label><label>每人减少 KB/s<input v-model.number="chainForm.rule.upload.stepKB" type="number" min="0"></label><label>最低 KB/s<input v-model.number="chainForm.rule.upload.minKB" type="number" min="0"></label></div></section><section class="rule-section"><div class="rule-heading"><label class="check"><input v-model="chainForm.rule.download.enabled" type="checkbox"><strong>限制下载</strong></label><small>关闭时应用不会接管下载速度</small></div><div v-if="chainForm.rule.download.enabled" class="rule-grid"><label>基础 KB/s<input v-model.number="chainForm.rule.download.baseKB" type="number" min="0"></label><label>每人减少 KB/s<input v-model.number="chainForm.rule.download.stepKB" type="number" min="0"></label><label>最低 KB/s<input v-model.number="chainForm.rule.download.minKB" type="number" min="0"></label></div></section><label class="check"><input v-model="chainForm.enabled" type="checkbox"><span>启用此绑定链</span></label><div class="modal-actions"><button type="button" @click="modal.chain=false">取消</button><button class="primary" :disabled="busy">保存绑定链</button></div></form></div>
    <div v-if="modal.confirm" class="modal-backdrop"><section class="modal compact confirm-modal"><div class="modal-head"><div><h2>{{confirmation.title}}</h2><p>{{confirmation.text}}</p></div><button @click="modal.confirm=false" v-html="icons.close"></button></div><div class="modal-actions"><button @click="modal.confirm=false">取消</button><button class="danger" @click="confirmation.run">删除</button></div></section></div>
  </main>`
})

application.config.errorHandler = error => {
  console.error('UI render error', error)
  window.__mediaThrottleBootError?.(error)
}
application.mount('#app')
window.__mediaThrottleUIReady = true
