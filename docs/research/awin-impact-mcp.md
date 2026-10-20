# Awin and impact.com MCP servers

Checked 20 October 2026. This note lists the tools named in each network's own docs and says whether those docs show a tool for disclosure, approval, or a placement receipt. "Unknown" means the page that was read does not say.

## Awin

Official server: `https://mcp.awin.com`.

Source: [Tools available in Awin MCP](https://help.awin.com/developers/docs/tools-available-in-awin-mcp) (published 30 September 2026) and [Awin MCP overview](https://help.awin.com/developers/docs/awin-mcp-overview).

The tools page says this first version is a curated set, not every screen in the Awin platform. It also says Awin MCP cannot approve or decline transactions, change account settings, or open every report.

| Tool | What the docs say |
| --- | --- |
| `awin_list_accounts` | Lists advertiser and publisher accounts on the login. |
| `awin_search_help` | Searches the Awin help center. |
| `awin_get_account_performance` | Advertiser account health for up to 31 days. |
| `awin_get_partner_performance` | Advertiser results by publisher for up to 31 days. |
| `awin_get_creative_performance` | Clicks, impressions, and commission by creative. |
| `awin_list_programmes` | Publisher programs by relationship. |
| `awin_get_programme_details` | One program's EPC, conversion rate, approval rate, and membership. |
| `awin_get_advertiser_performance` | Publisher clicks, impressions, sales, and commission by advertiser. |
| `awin_create_links` | Creates tracking links for joined advertisers. The assistant should show the links and ask the person to confirm. Up to 100 links per request. |
| `awin_compare_accounts` | Ranked snapshot across accounts the login can see. |
| `awin_get_portfolio_kpis` | KPI totals for those accounts. |

Disclosure: unknown. No tool on that page is described as adding or checking a disclosure.

Approval: the page says the connector cannot approve or decline transactions. `awin_get_programme_details` returns an approval rate, which is a figure, not an approval action. `awin_create_links` asks the person to confirm before the links are created. A placement approval queue is not described.

Receipts: unknown. No tool on that page is described as recording that a placement was published.

## impact.com

Official server: `https://mcp.impact.com/mcp`.

Sources: [MCP Quick Start](https://integrations.impact.com/ai-solutions/mcp-quick-start), [Partner MCP Tools](https://integrations.impact.com/ai-solutions/mcp-tools/partner-mcp-tools), and [Brand MCP Tools](https://integrations.impact.com/ai-solutions/mcp-tools/brand-mcp-tools). An account administrator turns MCP on. OAuth 2.1. The quick start says the assistant reads impact.com data under the user's existing permissions.

### Partner tools

| Tool | What the docs say |
| --- | --- |
| `search_help_center` | Searches the help center. |
| `get_marketing_guidance` | Strategy answers for the account type. |
| `query_performance` | Clicks, actions, and revenue. |
| `list_action_inquiries` | Lists inquiries about attribution or pay. Submitting a new inquiry stays in the platform UI. |
| `get_action_inquiry` | One inquiry, including resolution status. |
| `list_invoices` | Recipient-generated invoices. |
| `get_invoice` | One invoice and its line items. |
| `get_payout_status` | Payment eligibility, balance, and payout schedule. |
| `search_products` | Marketplace product search. |
| `list_partner_promo_codes` | Promo codes on the account. |
| `get_partner_promo_code` | One promo code. |
| `create_tracking_links` | Creates tracking links for program destinations. |
| `list_media_properties` | Sites and other channels, including ownership verification. |
| `recommend_programs` | Ranked program suggestions. |
| `evaluate_program_recommendation` | A scored fit for one program. |
| `list_tasks` | Open tasks. The page says it does not create or update tasks. |

### Brand tools

Brand accounts get `search_help_center`, `get_marketing_guidance`, `query_performance`, `list_action_inquiries`, `get_action_inquiry`, `list_invoices`, `get_invoice`, and `list_tasks` as well. The brand page adds:

| Tool | What the docs say |
| --- | --- |
| `get_website_metrics` | Traffic and audience for a domain. A sample prompt mentions checking a site before approving a partner. |
| `get_social_metrics` | Follower and engagement figures for a social profile. |
| `investigate_order_action` | Conversion records for one order id. |
| `get_brand_overdue_amount` | Overdue fees. |
| `recommend_partners` | Ranked partner suggestions. |
| `evaluate_partner_recommendation` | A scored fit for one partner. A sample prompt says "before I approve them." |
| `list_partners` | Partners already on the program, including pending. The page says this tool is not for approving or declining applications. |

The brand `list_action_inquiries` page says accepting, declining, and uploading resolutions stay in the platform UI.

Disclosure: unknown. No tool on those pages is described as adding or checking a disclosure.

Approval: no tool is described as approving a placement or a partner application. `list_partners` and `list_action_inquiries` say those decisions stay in the platform. `evaluate_partner_recommendation` and `get_website_metrics` can be asked before a person approves; they do not record the approval.

Receipts: unknown for a placement receipt. `list_invoices` and `get_invoice` are invoices, not a record that a placement was published.
