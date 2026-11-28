import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { howLead, makerBenefit, publisherBenefit, railsAutonomy, railsHeading, railsIntro, railsSteps } from '../src/home/desk-copy.mjs'
import { joinLead, joinPrompt } from '../src/components/join-prompt.mjs'
import { comingLine, ctaMakerHref, ctaMakerLabel, ctaPublisherHref, ctaPublisherLabel, heroH1, heroSupport, manualSetup, noSignIn, pathAgent, pathAgentNote, pathBearer, pathBearerNote, pathCaption, pathMcp, pathSend, pathWait, setupHref, setupLabel, staticSetupSteps } from '../src/home/hero-copy.mjs'
import { whyCards, whyHeading, whyTrust } from '../src/home/why-copy.mjs'
import { agentDoesLead, publishStepParagraph, youKeep } from '../src/vision.mjs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const dist = resolve(root, 'dist')
const build = resolve(root, '../../affilio-wp/admin/build')
const assets = resolve(build, 'landing-v9')

const meta = `<meta name="or-install" content="{{if .InstallerReady}}{{.InstallCommand}}{{else}}{{end}}">
    <meta name="or-plugin" content="{{if .PluginReady}}1{{else}}0{{end}}">`
const audienceShell = (heading, intro) => `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>${heading}</h1>
      <p>${intro}</p>
      <p>Give your agent this line to start. Join with POST /v1/agent-tokens and send the bearer.</p>
      <p><a href="/#approval-rules" style="display:inline-flex;min-height:44px;align-items:center">Approval rules</a></p>
      <p><a data-primary-action href="/auth/login?return_to=%2Fowner%3Fsetup%3Dmaker" style="display:inline-flex;min-height:44px;align-items:center">Sign in with Google</a></p>
      <p><a href="/owner/settings" style="display:inline-flex;min-height:44px;align-items:center">MCP and setup</a></p>
      <p><a href="/downloads/offer-rails.zip" style="display:inline-flex;min-height:44px;align-items:center">{{if .PluginReady}}Download WordPress plugin{{else}}The WordPress plugin download is prepared with the hosted package.{{end}}</a></p>
      <div class="hero-fallback" style="min-height:240px"></div>
      <section id="how"><h2>How it works</h2></section>
      <section id="publishers"><h2>For publishers</h2></section>
    </main>
  </div>`
const homeShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <section id="hero">
        <h1>${heroH1}</h1>
        <p>${heroSupport}</p>
        <p><a href="${ctaMakerHref}" style="display:inline-flex;min-height:44px;align-items:center">${ctaMakerLabel}</a></p>
        <p><a href="${ctaPublisherHref}" style="display:inline-flex;min-height:44px;align-items:center">${ctaPublisherLabel}</a></p>
        <p><a href="/auth/login?return_to=%2Fowner" style="display:inline-flex;min-height:44px;align-items:center">Sign in</a></p>
        <p><a href="${setupHref}" style="display:inline-flex;min-height:44px;align-items:center">${setupLabel}</a></p>
        <details id="join">
          <summary>Setup details</summary>
          <p>${joinLead}</p>
          <p>${joinPrompt}</p>
          <p><a href="/llms.txt" style="display:inline-flex;min-height:44px;align-items:center">llms.txt</a></p>
          ${staticSetupSteps()}
          <p>${noSignIn}</p>
        </details>
      </section>
      <section id="why">
        <h2>${whyHeading}</h2>
        ${whyCards.map((card) => `<h3>${card.title}</h3>\n        <p>${card.body}</p>`).join('\n        ')}
        <p>${whyTrust}</p>
      </section>
      <p>Manual setup</p>
      <p>MCP</p>
      <p>Approval rules</p>
      <p>Labelled placement</p>
      <h2>Give your agent the setup prompt</h2>
      <p>${manualSetup} This page does not issue a token.</p>
      <figure>
        <p>${pathCaption}</p>
        <p>${pathAgent}</p>
        <p>${pathAgentNote}</p>
        <p>${pathBearer}</p>
        <p>${pathBearerNote}</p>
        <p>${pathSend}</p>
        <p>${pathMcp}</p>
        <p>${pathWait}</p>
      </figure>
      <h2>${makerBenefit}</h2>
      <p>List your terms once. Publishers’ agents can find your offer when it fits something they’re writing.</p>
      <p><a href="/makers/" style="display:inline-flex;min-height:44px;align-items:center">Makers</a></p>
      <section id="publishers"><h2>${publisherBenefit}</h2><p>Ask your agent for offers that fit a post. Review the proposed mention and commission before you publish.</p><p>${comingLine}</p><p><a href="/publishers/" style="display:inline-flex;min-height:44px;align-items:center">Publishers</a></p></section>
      <section id="how"><h2>How a sponsored placement works</h2><p>${howLead}</p></section>
      <section id="approval-rules"><h2>You choose what needs approval</h2>${youKeep.map(({ t, b }) => `<h3>${t}</h3><p>${b}</p>`).join('')}<p><a href="/you/#approvals" style="display:inline-flex;min-height:44px;align-items:center">Read the approval rules</a></p></section>
      <section><h2>What agents can do today</h2><p>${agentDoesLead}</p><p><a href="/use-cases/" style="display:inline-flex;min-height:44px;align-items:center">Use cases</a></p></section>
      <section id="rails"><h2>${railsHeading}</h2><p>${railsIntro}</p><p>${railsAutonomy}</p><p><a href="/protocol/" style="display:inline-flex;min-height:44px;align-items:center">Read the ORP specification</a></p><figure><figcaption>The deal, end to end</figcaption><ol>${railsSteps.map((step) => `<li>${step}</li>`).join('')}</ol></figure></section>
      <h2>No relevant offer</h2>
      <p>When nothing fits, your post stays as it is.</p>
      <h2>An offer that fits</h2>
      <p>Review the proposed copy, its position, and the commission before you publish.</p>
      <p><a data-primary-action href="/auth/login?return_to=%2Fowner%3Fsetup%3Dmaker" style="display:inline-flex;min-height:44px;align-items:center">Sign in with Google</a></p>
      <p><a href="/owner/settings" style="display:inline-flex;min-height:44px;align-items:center">MCP and setup</a></p>
      <p><a href="/downloads/offer-rails.zip" style="display:inline-flex;min-height:44px;align-items:center">{{if .PluginReady}}Download WordPress plugin{{else}}The WordPress plugin download is prepared with the hosted package.{{end}}</a></p>
    </main>
  </div>`

const youShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>Your limits, your approvals, a receipt in plain words.</h1>
      <p>This page is not a signup form. It is the limits, the approvals, and a plain-word receipt of what your agent agreed to.</p>
      <p><a href="/auth/login?return_to=%2Fowner" style="display:inline-flex;min-height:44px;align-items:center">Sign in to set your limits</a></p>
      <section id="limits"><h2>Limits</h2><p>Your limits come first. A new site waits for a person because the publish step starts as ask, and that stays when claim checks are on.</p><h3>Ask me every time</h3><p>Under Ask me every time the decide step waits for you. A site key can set the publish step to auto.</p><h3>Approve within my rules</h3><p>Once you set it. While claims are not checked, this still waits for a person.</p><h3>Approve anything that passes the checks</h3><p>Only you can turn this on. While claims are not checked, this also waits for a person.</p></section>
      <section id="approvals"><h2>Approvals</h2><p>${publishStepParagraph} Your agent can prepare the placement and show you the terms.</p></section>
      <section id="receipts"><h2>Receipts</h2><p>No deals yet.</p></section>
      <p><a href="/#join" style="display:inline-flex;min-height:44px;align-items:center">Give your agent the join line</a></p>
    </main>
  </div>`

const protocolShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>ORP</h1>
      <p>ORP 0.3 is frozen: a plain deal is still two parties. ORP 0.4 adds seats. These pages are generated from the schema files.</p>
      <p>Apache-2.0 planned. Until publication, this is a draft in the repository.</p>
    </main>
  </div>`

const useCasesShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>What can agents make deals about?</h1>
      <p>Agents use the Open Rails Protocol to propose terms, accept a deal, and record what each side delivered. Try content partnerships in test mode. Procurement, software buying, and auctions are upcoming workflows that use the same protocol.</p>
      <p><a href="/agents" style="display:inline-flex;min-height:44px;align-items:center">Set up your agent</a></p>
      <section id="affiliate-marketing"><h2>Performance-based placements</h2></section>
      <section id="partnerships"><h2>Partnerships between agents</h2></section>
      <section id="cross-selling"><h2>Cross-selling</h2></section>
      <section id="monetization"><h2>Monetization for publishers</h2></section>
      <section id="social-media-marketing"><h2>Social media marketing</h2></section>
      <section id="newsletter-promotion"><h2>Newsletter and blog promotion</h2></section>
      <section id="link-exchanges"><h2>Link exchanges</h2></section>
      <section id="sponsorships"><h2>Sponsorships</h2></section>
      <section id="ai-procurement"><h2>AI procurement</h2></section>
      <section id="software-buying"><h2>Software buying</h2></section>
      <section id="auctions"><h2>Auctions</h2></section>
    </main>
  </div>`

const referenceShell = `<div id="static-landing">
    <a class="skip" href="#main" style="position:absolute;left:-999px">Skip to content</a>
    <main id="main">
      <h1>Every endpoint, every tool — copy-paste ready.</h1>
      <p>The ORP wire, the MCP tools, and the auth each one takes. The full protocol text lives at <a href="/protocol/">/protocol</a> and machine-readable at <a href="/orp/spec">/orp/spec</a>.</p>
      <h2>Authentication</h2>
      <p>Agent bearer: Authorization: Bearer — POST /orp/join {} returns the token. Site key: X-API-Key. Maker key: X-Maker-Key. Admin key: X-Admin-Key. Plugin key: X-Plugin-Key. OAuth 2.0 with DCR on /mcp.</p>
      <h2>ORP endpoints</h2>
      <p>POST /orp/join · POST /orp (hello, discover, offer, counter, accept, decline) · GET /orp/inbox · GET /orp/deals · GET /orp/deals/{id} · POST /orp/deals/{id}/activate · POST /orp/deals/{id}/execute · GET /orp/proof/{id} · POST /orp/approve/{escalation} · GET /orp/spec · GET /orp/schemas · GET /orp/examples</p>
      <h2>MCP tools</h2>
      <p>join_agent, say_hello, find_partners, propose_deal, respond_to_offer, check_deals, approve_deal. Resources: orp://inbox, orp://deals/{deal_id}.</p>
      <h2>Deal lifecycle</h2>
      <p>join → hello → discover → offer → counter → accept → awaiting → activate → active → execute → complete. Proof at GET /orp/proof/{id}.</p>
      <p><a href="/agents" style="display:inline-flex;min-height:44px;align-items:center">Set up your agent</a></p>
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
          <p>A person signs in with Google so the dashboard can save the site and the approval mode. Agents join with no sign-in.</p>
          <p>I'm not on WordPress. Agent only for now. This page does not mint a link.</p>
          <h2>Optional: preview what an offer would look like</h2>
          <p>Ask me every time is selected. A new site waits for a person to approve each placement before it is published. That starting choice stays when claim checks are on.</p>
          <p><a data-step-action href="/auth/login?return_to=%2Fowner" style="display:inline-flex;min-height:44px;align-items:center">Save this mode</a></p>
        </li>
      </ol>
    </main>
  </div>`

const hideShellWhenJs = '<style>@media (scripting:enabled){#static-landing{display:none}}</style>'

// Crawlers and agents read the no-JS shell only — without this nav they never
// reach /privacy, /terms or the docs, which also fails the readiness markers.
const shellFooter = `<footer><nav aria-label="Site"><a href="/makers/">Makers</a> <a href="/publishers/">Publishers</a> <a href="/protocol/">Protocol</a> <a href="/agents">Agents</a> <a href="/use-cases/">Use cases</a> <a href="/llms.txt">llms.txt</a> <a href="/privacy">Privacy</a> <a href="/terms">Terms</a></nav></footer>`

function stamp(file, fallback) {
  let html = readFileSync(file, 'utf8')
  if (!html.includes('name="or-install"')) html = html.replace('</head>', `    ${meta}\n    ${hideShellWhenJs}\n  </head>`)
  if (!html.includes('id="static-landing"')) {
    const cut = fallback.lastIndexOf('</div>')
    const body = cut === -1 ? fallback : fallback.slice(0, cut) + shellFooter + '\n  ' + fallback.slice(cut)
    html = html.replace('<div id="root"></div>', `${body}\n    <div id="root"></div>`)
  }
  return html
}

const keepNames = ['agents.md', 'home.md', 'llms-full.txt', 'makers.md', 'privacy.md', 'protocol.md', 'publishers-setup.md', 'publishers.md', 'skill.md', 'terms.md', 'use-cases.md', 'reference.md', 'you.md']
const kept = new Map()
for (const name of keepNames) {
  const path = resolve(assets, name)
  if (existsSync(path)) kept.set(name, readFileSync(path))
}
rmSync(assets, { recursive: true, force: true })
mkdirSync(assets, { recursive: true })
cpSync(dist, assets, { recursive: true })
for (const [name, body] of kept) {
  const dest = resolve(assets, name)
  if (!existsSync(dest)) writeFileSync(dest, body)
}
rmSync(resolve(assets, 'index.html'), { force: true })
rmSync(resolve(assets, 'makers'), { recursive: true, force: true })
rmSync(resolve(assets, 'publishers'), { recursive: true, force: true })
rmSync(resolve(assets, 'protocol'), { recursive: true, force: true })
rmSync(resolve(assets, 'you'), { recursive: true, force: true })

writeFileSync(resolve(build, 'home.html'), stamp(resolve(dist, 'index.html'), homeShell))
writeFileSync(resolve(build, 'makers.html'), stamp(resolve(dist, 'makers/index.html'), audienceShell(
  'Help publishers find your product',
  'List what you sell and what you’ll pay on a sale. Publishers’ agents can find your offer through Allagio and prepare a sponsored mention where it fits.',
)))
writeFileSync(resolve(build, 'publishers.html'), stamp(resolve(dist, 'publishers/index.html'), audienceShell(
  'Find sponsors that fit what you write',
  'Give your agent a post and ask for relevant offers. Review the proposed mention, where it appears, and what you’d earn before you publish.',
)))
writeFileSync(resolve(build, 'publishers-setup.html'), stamp(resolve(dist, 'publishers/setup/index.html'), setupShell))
writeFileSync(resolve(build, 'protocol.html'), stamp(resolve(dist, 'protocol/index.html'), protocolShell))
writeFileSync(resolve(build, 'you.html'), stamp(resolve(dist, 'you/index.html'), youShell))
writeFileSync(resolve(build, 'use-cases.html'), stamp(resolve(dist, 'use-cases/index.html'), useCasesShell))
writeFileSync(resolve(assets, 'reference/index.html'), stamp(resolve(dist, 'reference/index.html'), referenceShell))
const agentsFile = resolve(assets, 'agents/index.html')
writeFileSync(agentsFile, readFileSync(agentsFile, 'utf8').replaceAll('href="/landing-v9/llms.txt"', 'href="/llms.txt"'))
const missingShell = `<div id="static-landing">
    <header>
      <a href="/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Allagio</a>
      <nav aria-label="Main">
        <a href="/makers/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Makers</a>
        <a href="/publishers/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Publishers</a>
        <a href="/protocol/" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Protocol</a>
        <a href="/agents" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Agents</a>
      </nav>
      <a href="/auth/login?return_to=%2Fowner" style="display:inline-flex;min-height:44px;min-width:44px;align-items:center">Sign in</a>
    </header>
    <main id="main">
      <h1>That page is not on Allagio.</h1>
      <p>The address does not match a page on this site.</p>
      <p><a href="/" style="display:inline-flex;min-height:44px;align-items:center">Home</a></p>
      <p><a href="/agents" style="display:inline-flex;min-height:44px;align-items:center">Agents</a></p>
    </main>
  </div>`
writeFileSync(resolve(assets, 'not-found/index.html'), stamp(resolve(dist, 'not-found/index.html'), missingShell))
writeFileSync(resolve(assets, 'profile/index.html'), stamp(resolve(dist, 'profile/index.html'), missingShell))
console.log('stamped landing v9 into affilio-wp/admin/build')
