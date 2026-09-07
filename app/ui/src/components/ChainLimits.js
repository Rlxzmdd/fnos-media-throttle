// A chain's rule summary, not a claim that every downloader has already synced.
export default {
  props: { chain: { type: Object, required: true } },
  computed: {
    directions () {
      return ['upload', 'download'].map(key => ({ key, title: key === 'upload' ? '上传限制' : '下载限制', rule: this.chain.rule[key] }))
    }
  },
  methods: {
    current (rule) {
      if (!this.chain.enabled) return '未启用'
      if (!rule.enabled) return '未接管'
      if (this.chain.lastError) return '待同步'
      return Math.max(rule.minKB, rule.baseKB - Math.max(0, this.chain.viewerCount || 0) * rule.stepKB) + ' kb/s'
    }
  },
  template: `
    <div class="speed-pair rule-summary">
      <div v-for="direction in directions" :key="direction.key">
        <h4>{{ direction.title }}</h4>
        <dl class="rule-metrics">
          <div><dt>最大</dt><dd>{{ direction.rule.baseKB }} kb/s</dd></div>
          <div><dt title="每增加一位观众减少的限速">递减</dt><dd>{{ direction.rule.stepKB }} kb/s</dd></div>
          <div><dt>最小</dt><dd>{{ direction.rule.minKB }} kb/s</dd></div>
          <div title="当前为按最近人数计算的目标限速；实际执行结果以下载器状态为准"><dt>当前</dt><dd>{{ current(direction.rule) }}</dd></div>
        </dl>
      </div>
    </div>
  `
}
