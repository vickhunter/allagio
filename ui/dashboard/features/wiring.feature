# v6: wire setup + digest to PR #21 (vickhunter/affilio) at fb493f0. The PR's field names win (Product Lead).
# Source of truth: PR21-body-fb493f0.md + the handlers at fb493f0, read with the GitHub MCP (never cloned).
# Our rules are not bent to fit the PR; disagreements are open items in PARITY.md.
Feature: Real API mode against the PR #21 fb493f0 contract

  Background:
    Given the page is opened with ?api=real and an API base URL (a same-origin path or localhost)
    And no request ever goes to /v1/go/

  Scenario: Fixtures are the fb493f0 shapes
    Then every fixture in fixtures/api has exactly the keys the fb493f0 handler writes, in Go map (sorted) order
    And the hard-checks fixture and every 422 equal the PR hard-check table
    And every 409 is {reason, message} from the PR 409 table, including not_undone
    And no fixture has the old maker_commission placeholder

  Scenario: Digest loads without extra reads
    When the digest opens in real mode
    Then it reads GET /v1/account/digest and GET /v1/digest only, then records digest_opened
    And it makes no mode reads and no placement status reads on first load
    And Live items, each section's Live now and "N items went live since last digest" are the server's numbers
    And only items with live_now=true count toward Live now

  Scenario: Item fields
    Then each placement shows its post title, position, link text, label and clicks
    And "Approved by your rule: <policy_rule_text>" with the rules version, or "Approved by you"
    And a pending item says "Waiting for your post to update" with no Undo until the 4 s poll gives its status
    And a null or unknown plugin_state says "Status unknown"

  Scenario: Money per currency
    Then earnings show one line per currency and are never added together
    And null or an empty list shows "No earnings data yet"
    And offer sections say "Commission to pay" and "Not income"

  Scenario: Approval reason while the claim check is a stub
    Given approval_reason is claim_check_unavailable
    Then the digest shows approval_message at the top and next to each "Ask me every time"
    And setup disables auto and yolo with approval_message as the reason, for makers too (must-fix b)
    And nothing disabled is marked Suggested or checked

  Scenario: Paused comes with the digest
    Given paused.account is true
    Then the paused banner shows and nothing counts as live
    Given only paused.site is true
    Then that site's section shows a "Paused" pill

  Scenario: Redo refused because it was not undone
    When Redo answers 409 {reason:"not_undone", message}
    Then the toast says "Can't redo. The placement was not undone. Showing the latest status."
    And the item re-reads GET /v1/placements/:id

  Scenario: Retry of a failed restore fails a hard check (must-fix a)
    When Retry on a failed restore answers 422 {check_id, check_text}
    Then the card says "Can't put it back. <check_text> It stays removed."
    And Retry disappears and the item does not count as live

  Scenario: Makers can't retry
    Given a failed placement on a maker's offer
    Then there is no Retry button and the note says "Only the publisher can try again."

  Scenario: Key or agent context (must-fix c)
    Given the digest session is not signed in
    Then Undo and Pause still work
    And Redo, Retry of a failed restore and Resume are not offered
    And a plain line says those need the dashboard
    And on setup Continue is disabled with the same plain line

  Scenario: Long lists
    Given a site with 128 live items and a page of 50
    Then 6 show first, "Show all 50 items" shows the rest
    And "Showing the latest 50 items. 128 are live." links to the dashboard

  Scenario: Hard checks from the API
    Then setup renders exactly what GET /v1/policy/hard-checks returns, in order
    And if that call fails it says so in one line

  Scenario: Setup save
    Then each PUT /v1/setup/settings carries exactly the 13 fb493f0 keys
    And commission is {type: percent|fixed, value, currency} and listing_price is {value, currency} or null

  Scenario: Totals grid
    Then with any number of tiles every row is filled edge to edge, with no empty grey area

# v8: PR #21 @ 6042dcb
  Scenario: Latest 50 of N (real_latest_50_of_n, real_long_list)
    Given a site project with total_items 212 and 50 items in the digest
    Then the section says "Showing the latest 50 of 212. See them all in the dashboard"
    And a project whose total_items equals its items shows no note
  Scenario: Pause and resume use effective_mode (real_resume_effective_mode, real_pause_resume)
    When resume answers rows with effective_mode manual and an approval_message
    Then every mode reads "Ask me every time" and the outcome quotes the server's message
    When the rows say the claim checker is live
    Then each mode is the row's effective_mode
  Scenario: Setup save never changes the saved mode (setup_saved_mode_kept, real_setup_stub_modes)
    Given a saved setup
    Then the mode question is read-only with "Saving here keeps your saved approval mode"
  Scenario: First-time setup carries the agent's install_path (live smoke)
    Given the agent's link has ?install_path=cli
    Then GET /v1/setup/settings is called with install_path=cli
