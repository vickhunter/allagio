Feature: v7 design system on the D-032 screens (DESIGN.md)
  Keys, panels and states change how things look and move, never what the server is asked or what counts as live.
  Run at 390 and 1440. Axe 0. Zero /v1/go/ requests.

  Scenario: Keyboard list navigation                              # kb_list_navigation
    Given the digest
    When I press J, J, K, K
    Then focus moves to item 1, 2, 1, 1 with a 3px focus ring, and nothing is sent

  Scenario: R and A follow the existing rules                     # kb_guards
    Given the real digest with a focused live box
    When I press A
    Then nothing is sent and I am told approving happens in the dashboard
    When I press R
    Then exactly one revoke is sent, like pressing Undo
    When I press R again on the same item
    Then no restore is sent (R never redoes)
    And R on a listing sends nothing and says why

  Scenario: Key context                                           # kb_key_context
    Given the digest opened with a key
    Then R on the failed restore never restores or retries, and R on a live box still undoes

  Scenario: Typing is never intercepted                           # kb_guards, kb_typing_setup
    When I type "jkra?" in the palette or a setup field
    Then the text lands in the field, and no key action or overlay fires

  Scenario: Command palette                                       # palette_open_run_close
    When I press Ctrl+K
    Then the palette opens with focus in its input, and each command shows its keys
    And "Approve" is disabled with its reason
    When I press Esc, focus returns to the item I was on
    When I press Cmd+K, type "pause" and press Enter
    Then the pause confirmation opens and nothing is paused yet
    And a search with no match says so
    And ? opens the help with every shortcut, focus trapped, and Esc closes it

  Scenario: Item side panel                                       # panel_focus_trap
    When I press Enter on a focused item, or its Details button
    Then a side panel shows its facts with its own scroll, never money, never a link to the offer, never BLIP
    And at 390 it covers the full width
    And focus stays inside until Esc, which returns focus to where I was

  Scenario: Approval modes modal                                  # modal_focus_trap
    When I click "Change" on a site
    Then a centred modal with a blurred backdrop explains the modes, and nothing is saved
    And focus stays inside until Esc, which returns focus to "Change"

  Scenario: Reduced motion                                        # reduced_motion
    Given prefers-reduced-motion is reduce
    Then BLIP doesn't blink, the palette and keycap durations are about 0, and the screens are snapshotted

  Scenario: Keycap Continue                                       # keycap_continue
    Then setup has exactly one keycap, Continue, lifted 4px (6px on hover) with its reassurance line
    And the digest has no keycap

  Scenario: Setup can be skipped and resumed                      # continue_setup_pill
    When I press "Skip for now" in setup
    Then the digest shows one "Continue setup" pill naming the question I stopped at
    And the pill reopens setup at that question
    And saving setup removes the pill

  Scenario: Empty digest points to the next step                  # empty_state_action, digest_empty
    Then I see "All clear. Your next digest arrives …", BLIP, and exactly one action
    And "Copy the scan command" copies "npx offer-rails" and says so
    And BLIP appears nowhere else

  Scenario: Undo and kill-switch messages say what was and wasn't reverted   # undo_message_text
    Then the undo toast, pause and resume outcomes read exactly as in DESIGN.md §5

  Scenario: Skeleton matches the layout                           # skeleton_shape
    Then while loading, the skeleton totals have the same width, offset and columns as the loaded totals

  Scenario: Load error points to the next step                    # load_error_next_step
    Then I see "Nothing changed" and one Try again, no BLIP, and Try again loads the digest
