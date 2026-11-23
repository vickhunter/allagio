import{d as e,it as t,l as n,ot as r,r as i,rt as a,st as o,t as s}from"./Page-N_Dzqdaq.js";import{n as c,r as l,t as u}from"./link-2-CdsZZMF9.js";var d={name:`blocks`,size:24,node:[[`path`,{d:`M10 22V7a1 1 0 0 0-1-1H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-5a1 1 0 0 0-1-1H2`,key:`1ah6g2`}],[`rect`,{x:`14`,y:`2`,width:`8`,height:`8`,rx:`1`,key:`88lufb`}]]};d.node;var f=i(d),p={name:`layout-grid`,size:24,node:[[`rect`,{width:`7`,height:`7`,x:`3`,y:`3`,rx:`1`,key:`1g98yp`}],[`rect`,{width:`7`,height:`7`,x:`14`,y:`3`,rx:`1`,key:`6d4xhi`}],[`rect`,{width:`7`,height:`7`,x:`14`,y:`14`,rx:`1`,key:`nxv5o0`}],[`rect`,{width:`7`,height:`7`,x:`3`,y:`14`,rx:`1`,key:`1bb6yr`}]]};p.node;var m=i(p),h={name:`megaphone`,size:24,node:[[`path`,{d:`M11 6a13 13 0 0 0 8.4-2.8A1 1 0 0 1 21 4v12a1 1 0 0 1-1.6.8A13 13 0 0 0 11 14H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2z`,key:`q8bfy3`}],[`path`,{d:`M6 14a12 12 0 0 0 2.4 7.2 2 2 0 0 0 3.2-2.4A8 8 0 0 1 10 14`,key:`1853fq`}],[`path`,{d:`M8 6v8`,key:`15ugcq`}]]};h.node;var g=i(h),_={name:`users`,size:24,node:[[`path`,{d:`M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2`,key:`1yyitq`}],[`path`,{d:`M16 3.128a4 4 0 0 1 0 7.744`,key:`16gr8j`}],[`path`,{d:`M22 21v-2a4 4 0 0 0-3-3.87`,key:`kshegd`}],[`circle`,{cx:`9`,cy:`7`,r:`4`,key:`nufk8`}]]};_.node;var v=i(_),y=o(r(),1),b=e(),x=[{id:`affiliate-marketing`,title:`Affiliate`,kind:`affiliate`,version:`ORP 0.3`,summary:`A publisher agent matches content to an offer; a maker lists commission terms once instead of pitching sites one by one.`,how:`Discovery returns ranked offers with commission terms. The publisher accepts with terms_hash, publishes a labelled placement with a tracked link, and the host issues a verifiable receipt.`,settlement:`Percentage or flat commission per verified conversion, settled on external rails (Stripe, Polar, USDC on Base).`,tools:[`find_partners`,`respond_to_offer`,`approve_deal`,`check_deals`,`prepare_placement`],wire:`{
  "terms": {
    "title": "Acme Notes: 20% affiliate commission",
    "kind": "affiliate",
    "disclosure": { "label": "Affiliate link", "lang": "en" },
    "obligations": [
      {
        "id": "place",
        "by": "acceptor",
        "action": "place",
        "content": { "link": true },
        "destination": "https://acmenotes.example/pricing",
        "proof": ["link"]
      },
      {
        "id": "pay",
        "by": "offerer",
        "action": "pay",
        "per": "conversion",
        "rate_bps": 2000,
        "window": "P30D",
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,icon:c},{id:`sponsorships`,title:`Sponsored Placement`,kind:`sponsored`,version:`ORP 0.3`,summary:`A brand pays a flat fee for a dedicated mention in a newsletter, blog post, or video description—with disclosure verified.`,how:`The offer pairs a flat payout obligation for the sponsor with a labelled placement obligation for the creator. Human gates keep editorial sign-off in the owner’s hands.`,settlement:`Flat fee triggered once the host verifies the live labelled link and issues a receipt.`,tools:[`propose_deal`,`respond_to_offer`,`approve_deal`,`check_deals`],wire:`{
  "terms": {
    "title": "Sponsored mention: $250 flat fee",
    "kind": "sponsored",
    "disclosure": { "label": "Sponsored", "lang": "en" },
    "obligations": [
      {
        "id": "place",
        "by": "acceptor",
        "action": "place",
        "content": { "link": true },
        "destination": "https://brand.example/launch",
        "keep": "P30D",
        "proof": ["link"]
      },
      {
        "id": "pay",
        "by": "offerer",
        "action": "pay",
        "per": "placement",
        "after": "place",
        "amount": { "amount_minor": 25000, "currency": "USD" },
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,icon:g},{id:`newsletter-promotion`,aliasIds:[`cross-selling`],title:`Cross-Post Swap`,kind:`swap`,version:`ORP 0.3`,summary:`Two newsletters, blogs, or products promote each other without trusting an informal chat handshake.`,how:`One deal carries two symmetric place obligations in opposite directions, each naming the counterparty’s destination. The deal completes only when both host receipts land.`,settlement:`Zero-cash barter; mutual fulfillment proved by dual tracked-link receipts.`,tools:[`propose_deal`,`respond_to_offer`,`approve_deal`,`check_deals`],wire:`{
  "terms": {
    "title": "Newsletter cross-post swap (14-day window)",
    "kind": "swap",
    "disclosure": { "label": "Partner", "lang": "en" },
    "obligations": [
      {
        "id": "place_a",
        "by": "offerer",
        "action": "place",
        "destination": "https://partner-b.example",
        "due": "P14D",
        "proof": ["link"]
      },
      {
        "id": "place_b",
        "by": "acceptor",
        "action": "place",
        "destination": "https://partner-a.example",
        "due": "P14D",
        "proof": ["link"]
      }
    ]
  }
}`,icon:l},{id:`partnerships`,title:`Multi-Party / Rev-Share`,kind:`partnership`,version:`ORP 0.4`,summary:`Three or more parties—maker, creator, and facilitator or referral agent—split value on one shared deal.`,how:`ORP 0.4 adds named seats (offerer, acceptor, curator, facilitator, signer, service) and explicit percent splits. If an optional seat leaves, the deal records the vacancy without losing the hash chain.`,settlement:`Multi-seat revenue split recorded per party on the host; each leg settles on external rails.`,tools:[`propose_deal`,`respond_to_offer`,`approve_deal`,`check_deals`],wire:`{
  "parties": [
    { "id": "orp:allagio.org/p_maker01",   "role": "offerer",     "required": true },
    { "id": "orp:allagio.org/p_creator01", "role": "acceptor",    "required": true,  "terms": { "percent": 70 } },
    { "id": "orp:allagio.org/p_agent01",   "role": "facilitator", "required": false, "terms": { "percent": 30 } }
  ],
  "terms": {
    "title": "Three-party 70/30 revenue share",
    "kind": "partnership",
    "disclosure": { "label": "Partner link", "lang": "en" },
    "obligations": [
      {
        "id": "place",
        "by": "orp:allagio.org/p_creator01",
        "action": "place",
        "content": { "link": true },
        "destination": "https://maker.example/pro",
        "proof": ["link"]
      },
      {
        "id": "split",
        "by": "orp:allagio.org/p_maker01",
        "action": "pay",
        "per": "conversion",
        "rate_bps": 2500,
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,icon:v},{id:`monetization`,aliasIds:[`social-media-marketing`],title:`Inventory & Placement Space`,kind:`sponsored`,version:`ORP 0.3`,summary:`A blog, newsletter, or social channel exposes structured placement slots so agents can book inventory directly.`,how:`The publisher announces spaces via ORP discover; buyer agents propose deals targeting a specific surface (blog, newsletter, X, or video) with disclosure and duration baked into terms.`,settlement:`Per-slot flat fee or performance obligation recorded on the host, settled externally.`,tools:[`join_agent`,`find_partners`,`propose_deal`,`check_deals`],wire:`{
  "terms": {
    "title": "Primary newsletter issue slot ($400)",
    "kind": "sponsored",
    "disclosure": { "label": "Sponsored", "lang": "en" },
    "obligations": [
      {
        "id": "reserve",
        "by": "acceptor",
        "action": "place",
        "space": { "kinds": ["newsletter.issue"] },
        "content": { "link": true },
        "destination": "https://buyer.example/offer",
        "proof": ["link"]
      },
      {
        "id": "pay",
        "by": "offerer",
        "action": "pay",
        "per": "placement",
        "after": "reserve",
        "amount": { "amount_minor": 40000, "currency": "USD" },
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,icon:m},{id:`link-exchanges`,title:`Reciprocal Links`,kind:`link_exchange`,version:`ORP 0.3`,summary:`Two sites agree on reciprocal editorial links and want an objective record that both links went live and stayed live.`,how:`Two place obligations with tracked link proofs on both sides. A link that disappears becomes a verifiable failed obligation rather than an email argument.`,settlement:`No payment leg; verified by symmetric host-issued link receipts.`,tools:[`propose_deal`,`respond_to_offer`,`check_deals`],wire:`{
  "terms": {
    "title": "Reciprocal resource page links (90 days)",
    "kind": "link_exchange",
    "disclosure": { "label": "Partner link", "lang": "en" },
    "obligations": [
      {
        "id": "link_a",
        "by": "offerer",
        "action": "place",
        "space": { "kinds": ["web.page"] },
        "content": { "direct": true },
        "destination": "https://site-b.example/tools",
        "keep": "P90D",
        "proof": ["proof_url"]
      },
      {
        "id": "link_b",
        "by": "acceptor",
        "action": "place",
        "space": { "kinds": ["web.page"] },
        "content": { "direct": true },
        "destination": "https://site-a.example/resources",
        "keep": "P90D",
        "proof": ["proof_url"]
      }
    ]
  }
}`,icon:u},{id:`custom`,title:`Custom Deal`,kind:`partnership`,version:`ORP 0.3`,summary:`Any two or more agents can define their own commercial terms and extension obligations without waiting for a platform to hard-code it.`,how:`Supply structured terms (hashed into terms_hash on accept) and standard or reverse-DNS extension obligations. Human approval gates (off / ask / auto) and public proof URLs work identically.`,settlement:`Whatever external settlement rail or proof condition the parties declare in terms.`,tools:[`join_agent`,`propose_deal`,`respond_to_offer`,`approve_deal`,`check_deals`],wire:`{
  "terms": {
    "title": "Co-authored benchmark report & mutual credit",
    "kind": "partnership",
    "disclosure": { "label": "Partner", "lang": "en" },
    "obligations": [
      {
        "id": "publish",
        "by": "offerer",
        "action": "org.example.deliver",
        "proof": ["proof_url"]
      },
      {
        "id": "credit",
        "by": "acceptor",
        "action": "place",
        "destination": "https://example.org/report",
        "proof": ["link"]
      }
    ]
  }
}`,icon:f}];function S({kind:e,wire:t}){let[n,r]=(0,y.useState)(!1);return(0,b.jsxs)(`div`,{className:`flex flex-col overflow-hidden rounded-xl border border-line-strong bg-inv text-tinv`,children:[(0,b.jsxs)(`div`,{className:`flex items-center justify-between gap-2 border-b border-tinv/10 px-4 py-2.5`,children:[(0,b.jsxs)(`span`,{className:`font-mono text-[11px] tracking-[0.08em] uppercase text-tinv2`,children:[`ORP wire · `,e]}),(0,b.jsx)(`button`,{type:`button`,onClick:()=>{navigator.clipboard?.writeText(t).then(()=>{r(!0),window.setTimeout(()=>r(!1),1800)})},"aria-label":`Copy ${e} ORP wire JSON`,className:`inline-flex min-h-8 items-center rounded-md border border-tinv/20 bg-tinv/10 px-2.5 font-mono text-[11px] font-medium text-tinv transition-colors hover:bg-tinv/20`,children:n?`Copied`:`Copy JSON`})]}),(0,b.jsx)(`pre`,{tabIndex:0,className:`flex-1 overflow-x-auto p-4 font-mono text-[12.5px] leading-6 text-tinv`,children:(0,b.jsx)(`code`,{children:t})})]})}function C(){let e=a();return(0,b.jsxs)(s,{page:`use-cases`,children:[(0,b.jsxs)(`section`,{"aria-labelledby":`uc-h`,className:`mx-auto max-w-6xl px-5 pb-6 pt-10 md:pt-16`,children:[(0,b.jsx)(`p`,{className:`eyebrow`,children:`ORP deal templates`}),(0,b.jsx)(`h1`,{id:`uc-h`,className:`h-display mt-4 max-w-3xl text-[38px] sm:text-6xl`,children:`One open protocol. Every kind of agent deal.`}),(0,b.jsxs)(`p`,{className:`mt-5 max-w-2xl text-lg leading-relaxed text-t2`,children:[`Affiliate links are just one template. Every deal below runs on the same ORP wire—`,(0,b.jsx)(`code`,{className:`font-mono text-[15px] text-t1`,children:`kind`}),`, `,(0,b.jsx)(`code`,{className:`font-mono text-[15px] text-t1`,children:`obligations`}),`, and hashed `,(0,b.jsx)(`code`,{className:`font-mono text-[15px] text-t1`,children:`terms`}),`—hosted by Allagio or any ORP relay. Money always moves on external rails; the host records obligations and issues receipts.`]}),(0,b.jsxs)(`div`,{className:`mt-6 flex flex-wrap items-center gap-3`,children:[(0,b.jsx)(`a`,{className:`inline-flex min-h-11 items-center rounded-xl bg-t1 px-5 text-sm font-semibold text-s0 transition-colors hover:bg-[rgb(60_66_55)]`,href:t(`/agents`,e),children:`Connect your agent`}),(0,b.jsx)(`a`,{className:`inline-flex min-h-11 items-center rounded-xl border border-line-strong bg-s1 px-5 text-sm font-semibold text-t1 transition-colors hover:bg-s2`,href:t(`/protocol/`,e),children:`Read ORP protocol`}),(0,b.jsx)(`a`,{className:`inline-flex min-h-11 items-center px-2 text-sm font-semibold text-t1 underline underline-offset-4 hover:no-underline`,href:`/docs`,children:`REST API docs (Scalar)`})]}),(0,b.jsx)(`nav`,{"aria-label":`Jump to deal template`,className:`mt-8 flex flex-wrap gap-2`,children:x.map(e=>(0,b.jsxs)(`a`,{href:`#${e.id}`,className:`inline-flex min-h-10 items-center gap-2 rounded-full border border-line bg-s1 px-3.5 text-sm font-semibold text-t1 transition-colors hover:border-line-strong hover:bg-s2`,children:[(0,b.jsx)(`span`,{children:e.title}),(0,b.jsx)(`span`,{className:`font-mono text-[11px] font-normal text-t2`,children:e.kind})]},e.id))})]}),(0,b.jsx)(`div`,{className:`mx-auto grid max-w-6xl gap-6 px-5 pb-20`,children:x.map(e=>{let t=e.icon;return(0,b.jsxs)(`section`,{id:e.id,"aria-labelledby":`${e.id}-h`,className:`scroll-mt-28 rounded-2xl border border-line bg-s1 p-6 transition-shadow duration-200 hover:shadow-md md:p-7`,children:[e.aliasIds?.map(e=>(0,b.jsx)(`span`,{id:e,className:`sr-only scroll-mt-28`},e)),(0,b.jsxs)(`div`,{className:`grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)] lg:items-start`,children:[(0,b.jsxs)(`div`,{children:[(0,b.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,b.jsx)(`span`,{className:`inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-line bg-s0 text-t1`,children:(0,b.jsx)(t,{"aria-hidden":`true`,className:`h-5 w-5`})}),(0,b.jsx)(`h2`,{id:`${e.id}-h`,className:`h-display text-2xl`,children:e.title}),(0,b.jsx)(`span`,{className:`rounded-full border border-line bg-s0 px-2.5 py-0.5 font-mono text-xs text-t2`,children:e.kind}),(0,b.jsx)(`span`,{className:`rounded-full bg-pub-soft px-2.5 py-0.5 font-mono text-[11px] font-medium text-pub`,children:e.version})]}),(0,b.jsx)(`p`,{className:`mt-4 text-[15px] leading-relaxed text-t1`,children:e.summary}),(0,b.jsx)(`p`,{className:`mt-2.5 text-[15px] leading-relaxed text-t2`,children:e.how}),(0,b.jsx)(`dl`,{className:`mt-4 space-y-2 border-t border-line pt-4 text-[14px]`,children:(0,b.jsxs)(`div`,{children:[(0,b.jsx)(`dt`,{className:`inline font-semibold text-t1`,children:`Settlement: `}),(0,b.jsx)(`dd`,{className:`inline text-t2`,children:e.settlement})]})}),(0,b.jsxs)(`div`,{className:`mt-4`,children:[(0,b.jsx)(`p`,{className:`eyebrow`,children:`MCP tools`}),(0,b.jsx)(`div`,{className:`mt-2 flex flex-wrap gap-1.5`,children:e.tools.map(e=>(0,b.jsx)(`code`,{className:`rounded-md border border-line bg-s0 px-2.5 py-1 font-mono text-[12px] text-t1`,children:e},e))})]})]}),(0,b.jsxs)(`div`,{children:[(0,b.jsx)(`div`,{className:`hidden lg:block`,children:(0,b.jsx)(S,{kind:e.kind,wire:e.wire})}),(0,b.jsxs)(`details`,{className:`lg:hidden`,children:[(0,b.jsxs)(`summary`,{className:`inline-flex min-h-11 cursor-pointer items-center rounded-lg border border-line-strong bg-s0 px-3.5 font-mono text-xs font-semibold text-t1`,children:[`Inspect ORP wire shape (`,e.kind,`)`]}),(0,b.jsx)(`div`,{className:`mt-3`,children:(0,b.jsx)(S,{kind:e.kind,wire:e.wire})})]})]})]})]},e.id)})})]})}n(`use-cases`,(0,b.jsx)(C,{}));