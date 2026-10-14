Feature: Digest, all my projects and sites on one page (D-032)

  Background:
    Given the viewport is 1440 wide or 390 wide
    And every request to "**/v1/go/**" is aborted

  Scenario: Digest opens and totals show first
    Given the digest fixture "normal"
    When I open the digest page
    Then an "offerrails:digest-opened" event fires with the digest_id
    And account totals (live items, clicks, earnings in test mode, projects, sites) are above the list
    And there is one section per project and per site
    And no box preview link points anywhere but "#preview"

  Scenario: Undo revokes immediately, then redo restores on the server
    When I press "Undo" on a live item on my site
    Then "POST /v1/placements/:id/revoke" with reason "undo" is sent right away, with no client-side hold
    And the item shows "Removed · leaving your post shortly"
    And a toast with "Redo" appears without a modal for 6 seconds
    When I press "Redo"
    Then "POST /v1/placements/:id/restore" is sent and on 200 the item shows "Going back into your post"
    And when the plugin reports placed, the item shows "Live"
    And "Live now" for the site, the project and the account totals drop by one after the undo and come back after the restore

  Scenario: Undo then close the page
    When I press "Undo" and close the tab at once
    Then the revoke request was already sent and the placement stays revoked

  Scenario Outline: Redo refused by the server
    Given restore answers <status> with <body>
    When I press "Undo" then "Redo"
    Then I see "<copy>" and the item stays removed, never raw JSON
    Examples:
      | status | body                         | copy                                                              |
      | 409    | reason window_expired (7 s)  | Too late to redo. The box stays removed.                          |
      | 409    | reason paused                | Can't redo: you paused after undoing. The box stays removed.     |
      | 409    | no reason                    | Can't redo this one. The box stays removed.                       |
      | 403    | not the person who undid it  | Only the person who removed this box can bring it back.           |
      | 422    | cap + check_text             | the server's check_text, as is                                    |
      | 422    | relevance_min + check_text   | the server's check_text, as is                                    |
      | 422    | offer_live + check_text      | the server's check_text, as is                                    |
      | 422    | disclosure, no check_text    | Can't redo. The disclosure label could not be added. ...          |
      | 422    | unknown id, no check_text    | Can't redo: it no longer passes the checks. ...                   |

  Scenario Outline: Plugin progress after undo is told honestly
    Given the plugin reports <plugin_state>
    Then the item shows "<copy>"
    Examples:
      | plugin_state | copy                                                                                            |
      | pending      | Removed · leaving your post shortly                                                             |
      | removed      | Box removed                                                                                     |
      | offline      | Your site is offline. Clicks already stop at the link; the box will come out when it reconnects. |
      | failed       | Your site couldn't update the post. Check the notice in your WordPress admin. (warning style)   |
      | null/unknown | Status unknown                                                                                  |
      | placed       | Live · <date>                                                                                    |

  Scenario: Polling schedule with a fake clock
    Given the plugin stays pending after an undo
    Then the page asks every 4 s for the first 60 s, then every 30 s
    And after 10 minutes it stops and says "Still waiting for your site"
    And while the tab is hidden it does not ask, and it asks again as soon as the tab is visible

  Scenario: Approval rule shown in plain words
    Then each item shows policy_rule_text and carries policy_rule_id

  Scenario: Earnings zero with data
    Then a site with data and no sales shows "€0.00"

  Scenario: Pause everything with confirm
    When I press "Pause everything"
    Then a confirm step appears and nothing is paused yet
    When I confirm
    Then "POST /v1/account/pause" with scope "all" is sent, "offerrails:pause-all" fires and every section shows "Ask me every time"
    And a "Resume" control marked signed-in only appears

  Scenario: Resume needs a confirm and a signed-in session
    Given everything is paused
    When I press "Resume" and confirm
    Then "POST /v1/account/resume" with scope "all" is sent and "offerrails:resume-all" fires with session_only true
    But when not signed in, Resume is disabled and says to sign in
    And when the server answers 403, I see plain copy and everything stays paused

  Scenario: Empty digest
    Given the digest fixture "empty"
    Then I see "All clear." with when the next digest arrives, exactly one action, and no digest items

  Scenario: Many items and the 140-character cap
    Given the digest fixture "many" with 12 or more items and copy at 140 characters
    Then every box copy fits its card without horizontal overflow

  Scenario: Earnings data missing
    Given the digest fixture "no-earnings"
    Then earnings read "No earnings data yet" and never "0" or "€0.00"

  # v4: failed plugin updates and Retry (spec 03:35-03:39, failed_action removed|restored)
  Scenario: Failed removal shows the reason and a Retry button
    Given an item with plugin_state "failed", failed_action "removed" and a plugin_failed_reason
    Then the amber note says "Your site couldn't update the post. Check the notice in your WordPress admin. Clicks already stop at the link."
    And the reason is shown word for word under it in small text
    And a "Try removing again" button at least 44 px tall is inside the note

  Scenario: Failed restore with no reason
    Given an item with plugin_state "failed", failed_action "restored" and plugin_failed_reason null
    Then the note shows no reason line and a "Try putting it back again" button

  Scenario: Retry a failed removal
    When I press "Try removing again"
    Then "POST /v1/placements/:id/retry" is sent once, even if I press twice, and the button shows a busy state
    And the item shows "Removed · leaving your post shortly" and polling starts again
    When the server reports removed
    Then the item shows "Box removed", never "Live", and "Live now" is unchanged
    And a retried removal that reports "placed" shows "Status unknown", never "Live"

  Scenario: Retry a failed restore refused by a check
    Given retry answers 422 with check_text
    When I press "Try putting it back again"
    Then the check_text is shown word for word, the item stays failed and Retry is offered again

  Scenario: Retry after the placement moved on
    Given retry answers 409 with reason "not_failed"
    When I press Retry
    Then I see "This box already changed. Showing the latest status." and the item is refetched and shows its latest status

  Scenario: Unrecognised 409 or 422 shows the server text
    Given retry or restore answers 409/422 with an unknown reason and a server message
    Then the server text is shown word for word, with a generic sentence when there is no text

  # v4.1 (PL/AI Engineer 03:43)
  Scenario: A failed restore is not live
    Given I undo a live item on my site and "Live now" drops by one
    When I press "Redo" and the restore is accepted
    Then "Live now" goes up by one while it goes back into the post
    When the plugin reports failed with failed_action "restored"
    Then "Live now" is back to the undone value
    And the note says "Clicks stay off until it's back in your post."
    When I press "Try putting it back again" and the server accepts
    Then "Live now" stays at the undone value while pending
    When the plugin reports placed
    Then "Live now" goes up by one and the item shows "Live"

  Scenario: Give up on a failed restore
    Given a failed restore
    Then the item offers "Leave it removed" instead of "Undo"
    When I press "Leave it removed"
    Then "POST /v1/placements/:id/revoke" with reason "undo" is sent, "Live now" does not change and no Redo toast appears

  Scenario: Busy Retry stays readable
    When a retry is running
    Then the button keeps full-contrast text with a spinner, not a faded button

  Scenario: 409 text comes from message
    Given a 409 with an unknown reason and a message
    Then the message is shown word for word

  # v4.2
  Scenario: A failed restore does not look live
    Given an item with plugin_state "failed" and failed_action "restored"
    Then its box preview has the dashed, muted not-live look (readable, no strikethrough)
    And a failed removal keeps the removed look with strikethrough

  # v4.3 (final: PL / AI Engineer / Site Keeper, 03:50). Rule: never show removed or placed before the server confirms it.
  Scenario: Leave it removed only for a failed restore
    Given a failed restore and a failed removal
    Then only the failed restore offers "Leave it removed"
    And the failed removal offers only "Try removing again"

  Scenario: Leave it removed waits for the plugin
    When I press "Leave it removed"
    Then the item shows "Taking it out of your post" and clicks are off at once
    And it shows "Box removed" only after the server reports plugin_state removed
    And if the plugin fails, it becomes a failed removal with only "Try removing again"

  Scenario: Nothing is shown as removed or placed before the server confirms it
    Given every action: undo, redo (restore), retry a removal, retry a restore, leave it removed, pause, resume
    Then while the server has not confirmed, no state line, note or toast says "removed" or "Live"
    And pause and resume show "Pausing…" / "Resuming…" until the server answers
    And "Live now" may drop at once on undo, but rises only when the plugin confirms placed

  Scenario Outline: Pending copy
    Then the state line reads "<copy>" while plugin_state is pending after <action>
    Examples:
      | action            | copy                        |
      | undo              | Leaving your post shortly   |
      | leave it removed  | Taking it out of your post  |
      | redo / retry back | Going back into your post   |
      | retry a removal   | Leaving your post shortly   |

  # v9: PR #21 @ 2d3aed7. review_reason + message are optional fields; at 2d3aed7 they ride maker routes (GET /v1/listings),
  # not the owner digest, so the digest shows them only when present.
  Scenario Outline: A listing held in review says which check, the server's words, and a next step (listing_in_review_<reason>)
    Given an offer item with status "submitted", review_reason "<reason>" and a message
    Then its state reads "In review" and there is no Undo
    And "Check holding it:" says "<check>"
    And "Server says:" quotes the server message word for word
    And "Next step:" says what to do, and no time is promised
    Examples:
      | reason    | check                                                           |
      | claims    | Claims are not checked yet.                                      |
      | head      | The offer page did not answer.                                   |
      | relevance | The offer scored below the minimum relevance.                   |
      | caps      | You already have as many offers listed as the cap allows.       |
  Scenario: Hosted runs CLAIMS_CHECK=stub (listing_in_review_claims)
    Then an untrusted maker's offer stays "In review" on claims, reads in plain words, and says "Nothing to do for now." first
  Scenario: Review fields are optional (listing_review_optional)
    When review_reason or message is missing or null, the row is a plain listing with no review block

  Scenario: Who undid it, from the owner id (digest_undone_by_owner_id)
    Given a digest item with undone_by set to an owner id (2d3aed7)
    Then the row and its details panel say "Undone by an owner on this account" and never show the id
    And an empty undone_by shows nothing
