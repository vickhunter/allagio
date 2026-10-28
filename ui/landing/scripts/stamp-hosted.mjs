import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const dist = resolve(root, 'dist')
const build = resolve(root, '../../affilio-wp/admin/build')
const assets = resolve(build, 'landing-v9')

const meta = `<meta name="or-install" content="{{if .InstallerReady}}{{.InstallCommand}}{{else}}{{end}}">
    <meta name="or-plugin" content="{{if .PluginReady}}1{{else}}0{{end}}">`
const shell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>Affiliate marketing, evolved for agents.</h1>
      <p><a data-primary-action href="/auth/login?return_to=%2Fowner%3Fsetup%3Dmaker" style="display:inline-flex;min-height:44px;align-items:center">Sign in with Google</a></p>
      <p><a href="/owner/settings" style="display:inline-flex;min-height:44px;align-items:center">MCP and setup</a></p>
      <p><a href="/downloads/offer-rails.zip" style="display:inline-flex;min-height:44px;align-items:center">{{if .PluginReady}}Download WordPress plugin{{else}}The WordPress plugin download is prepared with the hosted package.{{end}}</a></p>
      <div class="hero-fallback" style="min-height:240px"></div>
      <section id="how"><h2>How it works</h2></section>
      <section id="publishers"><h2>For publishers</h2></section>
    </main>
  </div>`

const youShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>Your limits, your approvals, a receipt in plain words.</h1>
      <p>This page is not a signup form. It is the limits, the approvals, and a plain-word receipt of what your agent agreed to.</p>
      <p>Your agent joins with POST /v1/agent-tokens and a bearer. No sign-in link.</p>
      <section id="limits"><h2>Limits</h2><p>Your limits come first. A new site opens on Manual: ask every time.</p><h3>Ask every time</h3><p>Manual. A new site opens here. Each placement waits for you.</p><h3>Approve within your rules</h3><p>Once you set it. While claims are not checked, this still waits for a person.</p><h3>Approve anything that passes the nine checks</h3><p>Only you can turn this on. While claims are not checked, this also waits for a person.</p></section>
      <section id="approvals"><h2>Approvals</h2><p>Ask me every time, a person approves each placement. In Approve within my rules and Approve anything that passes the checks, the owner's rule decides, and agent tools still cannot change that rule.</p></section>
      <section id="receipts"><h2>Receipts</h2><p>Example wording. Not a live agreement.</p><p>Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.</p></section>
    </main>
  </div>`

const protocolShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>Offer Rails Protocol</h1>
      <p>ORP 0.3 is frozen: a plain deal is still two parties. ORP 0.4 adds seats. These pages are generated from the schema files.</p>
      <p>Apache-2.0 planned. Until publication, this is a draft in the repository.</p>
    </main>
  </div>`

const setupShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>Set up your site.</h1>
      <ol>
        <li>
          <h2>Install the plugin</h2>
          <p>To do</p>
          <p><a data-step-action href="/downloads/offer-rails.zip" style="display:inline-flex;min-height:44px;align-items:center">Download the plugin</a></p>
        </li>
        <li>
          <h2>Connect your site</h2>
          <p>To do</p>
          <p><a data-step-action href="/auth/login?return_to=%2Fowner%2Fconnect-site" style="display:inline-flex;min-height:44px;align-items:center">Connect your site</a></p>
        </li>
        <li>
          <h2>Choose the approval mode</h2>
          <p>To do</p>
          <p>Manual is selected. A new site waits for a person to approve each placement before it is published. That starting choice stays when claim checks are on.</p>
          <p><a data-step-action href="/auth/login?return_to=%2Fowner" style="display:inline-flex;min-height:44px;align-items:center">Save this mode</a></p>
        </li>
      </ol>
    </main>
  </div>`

function stamp(file, fallback = shell) {
  let html = readFileSync(file, 'utf8')
  if (!html.includes('name="or-install"')) html = html.replace('</head>', `    ${meta}\n  </head>`)
  if (!html.includes('id="static-landing"')) html = html.replace('<div id="root"></div>', `${fallback}\n    <div id="root"></div>`)
  return html
}

rmSync(assets, { recursive: true, force: true })
mkdirSync(assets, { recursive: true })
cpSync(dist, assets, { recursive: true })
rmSync(resolve(assets, 'index.html'), { force: true })
rmSync(resolve(assets, 'makers'), { recursive: true, force: true })
rmSync(resolve(assets, 'publishers'), { recursive: true, force: true })
rmSync(resolve(assets, 'protocol'), { recursive: true, force: true })
rmSync(resolve(assets, 'you'), { recursive: true, force: true })

writeFileSync(resolve(build, 'home.html'), stamp(resolve(dist, 'index.html')))
writeFileSync(resolve(build, 'makers.html'), stamp(resolve(dist, 'makers/index.html')))
writeFileSync(resolve(build, 'publishers.html'), stamp(resolve(dist, 'publishers/index.html')))
writeFileSync(resolve(build, 'publishers-setup.html'), stamp(resolve(dist, 'publishers/setup/index.html'), setupShell))
writeFileSync(resolve(build, 'protocol.html'), stamp(resolve(dist, 'protocol/index.html'), protocolShell))
writeFileSync(resolve(build, 'you.html'), stamp(resolve(dist, 'you/index.html'), youShell))
const agentsFile = resolve(assets, 'agents/index.html')
writeFileSync(agentsFile, readFileSync(agentsFile, 'utf8').replaceAll('href="/landing-v9/llms.txt"', 'href="/llms.txt"'))
const missingShell = `<div id="static-landing">
    <header>
      <a href="/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Offer Rails</a>
      <nav aria-label="Main">
        <a href="/makers/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Makers</a>
        <a href="/publishers/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Publishers</a>
        <a href="/protocol/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Protocol</a>
        <a href="/agents" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Agents</a>
      </nav>
      <a href="/auth/login?return_to=%2Fowner" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Sign in</a>
    </header>
    <main id="main">
      <h1>That page is not on Offer Rails.</h1>
      <p>The address does not match a page on this site.</p>
      <p><a href="/" style="display:inline-flex;min-height:44px;align-items:center">Home</a></p>
      <p><a href="/agents" style="display:inline-flex;min-height:44px;align-items:center">Agents</a></p>
    </main>
  </div>`
writeFileSync(resolve(assets, 'not-found/index.html'), stamp(resolve(dist, 'not-found/index.html'), missingShell))
writeFileSync(resolve(assets, 'profile/index.html'), stamp(resolve(dist, 'profile/index.html'), missingShell))
console.log('stamped landing v9 into affilio-wp/admin/build')
