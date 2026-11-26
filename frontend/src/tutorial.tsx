import { Icon, PageHeader } from './components'

const purchaseURL = 'https://portal.bigmess.org/'

const steps = [
  {
    title: '1. 先买一个 Clash 订阅',
    body: 'AI Cage 只负责“调度”，不自带机场节点。你要先有自己的 Clash 订阅，然后再导入到 AI Cage。',
    extra: <a className="button button-primary" href={purchaseURL} target="_blank" rel="noreferrer"><Icon name="external" size={16}/>去 BigMess 购买订阅</a>,
  },
  {
    title: '2. 导入订阅',
    body: '打开“订阅”页，点“导入 Clash Verge 当前配置”。AI Cage 会只读导入 Verge 当前启用的配置快照，并自动切换为独立内核模式。',
  },
  {
    title: '3. 关掉 Verge 的 TUN',
    body: 'AI Cage 和 Clash Verge 不能同时开 TUN。你要先在 Verge 里关掉 TUN，再回到 AI Cage 点“启动代理”。',
  },
  {
    title: '4. 只看 AI-CAGE 这个组',
    body: '沙盒流量只认 AI-CAGE。你在其他组里换节点，不会影响沙盒出口，除非 AI-CAGE 正好指向那个组。',
  },
  {
    title: '5. 打开沙盒窗口',
    body: '到“应用”页点“打开沙盒窗口”。AI Cage 会创建一个独立宿主窗口，把应用主窗口嵌进去；关闭宿主窗口时应用也会结束。',
  },
]

const faqs = [
  {
    q: '为什么我可以“选中多个节点”？',
    a: 'Clash 的每个代理组各自都有一个“当前节点”。你在 A 组选了一个，在 B 组也可以再选一个，它们互不影响。真正决定沙盒出口的，是 AI-CAGE 这一个组当前指向哪个节点或上游组。',
  },
  {
    q: '我换了节点，为什么出口 IP 还是没变？',
    a: '99% 是因为你改的是别的组，而不是 AI-CAGE。请先在“节点”页左侧选中 AI-CAGE，再在右侧选具体节点。只有这样，沙盒出口才会跟着变。',
  },
  {
    q: '沙盒窗口是不是安全沙盒？',
    a: '不是。它是一个独立的窗口托管壳，用来把应用“包”在 AI Cage 里，不提供文件系统隔离，也不解除应用自身的系统沙盒。',
  },
  {
    q: '能不能防止 Claude 检测或封号？',
    a: '不能，也不会提供这类功能。请遵守平台条款，使用官方客户端，不要共享账号或尝试规避风控。',
  },
]

export function Tutorial() {
  return <>
    <PageHeader eyebrow="GETTING STARTED" title="新手教程" description="从买订阅到打开沙盒窗口，按顺序做完就能用。"/>
    <div className="tutorial-grid">
      {steps.map(step => <section className="panel tutorial-card" key={step.title}>
        <div className="tutorial-icon"><Icon name="info" size={24}/></div>
        <h3>{step.title}</h3>
        <p>{step.body}</p>
        {step.extra}
      </section>)}
    </div>
    <div className="tutorial-faq">
      <h2>常见问题</h2>
      {faqs.map(item => <details key={item.q}>
        <summary>{item.q}</summary>
        <p>{item.a}</p>
      </details>)}
    </div>
    <div className="info-banner tutorial-warning">
      <Icon name="warning"/>
      <p><strong>合规提醒</strong>AI Cage 只做本地代理调度和窗口托管，不提供规避检测、封号或绕过平台风控的能力。请遵守你使用的每个网站或服务的用户协议。</p>
    </div>
  </>
}
