// Presentational settings view. Persistence and errors remain in the controller.
export default {
  props: ['settings', 'theme', 'busy'],
  emits: ['update:theme', 'save', 'export-config', 'import-config'],
  data: () => ({
    themes: [{ value: 'system', label: '跟随系统' }, { value: 'light', label: '浅色' }, { value: 'dark', label: '深色' }],
    releaseModes: [
      { value: 'original', label: '恢复原值' },
      { value: 'unlimited', label: '解除限速' },
      { value: 'maximum', label: '设为最高限速' }
    ]
  }),
  methods: {
    chooseConfig () { this.$refs.configFile?.click() },
    importSelected (event) {
      const file = event.target.files?.[0]
      event.target.value = ''
      if (file) this.$emit('import-config', file)
    }
  },
  template: `
    <section class="settings-card">
      <div><h2>外观</h2><p>跟随飞牛系统主题，或手动选择应用外观。</p></div>
      <div class="theme-switch" role="group" aria-label="外观主题">
        <button v-for="option in themes" :key="option.value"
          :class="{active: theme === option.value}" :aria-pressed="theme === option.value"
          @click="$emit('update:theme', option.value)">{{ option.label }}</button>
      </div>
    </section>
    <section class="settings-card release-settings">
      <div>
        <h2>退出或解除绑定时</h2>
        <p>仅处理已接管的上传、下载方向；也适用于禁用下载器或停止接管某个方向。</p>
        <p>最高限速取最后启用绑定链的基础速度。默认恢复接管前原值。</p>
        <p>关闭窗口不会停止后台服务；强制终止或断电无法立即执行释放。</p>
      </div>
      <div class="theme-switch" role="group" aria-label="退出限速策略">
        <button v-for="option in releaseModes" :key="option.value" :disabled="busy"
          :class="{active: settings.releaseMode === option.value}"
          :aria-pressed="settings.releaseMode === option.value"
          @click="$emit('save', {releaseMode: option.value})">{{ option.label }}</button>
      </div>
    </section>
    <section class="settings-card">
      <div><h2>Debug 模式</h2><p>记录适配器诊断信息；不会记录密码、API Key 或登录令牌。</p></div>
      <label class="switch">
        <input :checked="settings.debug" :disabled="busy" type="checkbox" aria-label="Debug 模式"
          @change="$emit('save', {debug: $event.target.checked})"><span></span>
      </label>
    </section>
    <section class="settings-card backup-settings">
      <div>
        <h2>配置备份</h2>
        <p>导出设置、影视库、下载器和绑定链，可用于迁移或恢复；运行日志不会写入备份。</p>
        <p>当前应用版本：v{{settings.appVersion || '开发版'}}</p>
        <p class="backup-warning">备份包含下载器密码和影视库 API Key，请勿公开分享。</p>
      </div>
      <div class="backup-actions">
        <button type="button" class="backup-button secondary" :disabled="busy" @click="$emit('export-config')">导出配置</button>
        <button type="button" class="backup-button primary" :disabled="busy" @click="chooseConfig">导入配置</button>
        <input ref="configFile" class="visually-hidden" type="file" accept="application/json,.json" @change="importSelected">
      </div>
    </section>
    <section class="settings-card about">
      <div><div><h2>观影联动限速</h2><p>FNOS 原生应用 · 绑定链架构 · v{{settings.appVersion || '开发版'}}</p></div></div>
      <small>作者：isZhous</small>
    </section>
  `
}
