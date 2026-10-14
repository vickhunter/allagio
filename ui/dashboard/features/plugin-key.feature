# v8: plugin key UI (Product Lead must-fix A), wired to PR #21 @ 6042dcb handlers/plugin_keys.go.
# v9: errors re-wired to 2d3aed7 writePluginKeyError {error, reason, message}: `reason` picks the message, `message` is shown as the server's words,
#     the v8 text is only the fallback when a body has no reason (`error` kept for compat).
# Tests: tests/run_tests.py (mocked, 1440 + 390) and tests/live_smoke_6042dcb.py (local 6042dcb API).
Feature: Plugin key, shown once
  The WordPress plugin needs its own key to confirm boxes. The owner sees it exactly once and knows where to paste it.

  Scenario: Shown once after a site connects (key_setup_shown_once)
    Given a publisher saves setup in the dashboard and their site has no active plugin key
    When the save succeeds
    Then the page asks the server for a new key with POST /v1/sites/:id/plugin-key
    And shows it in a monospace field with a Copy button
    And says "We won't show this key again." and where to paste it: Affilio › Settings › Plugin key
    And the keycap Continue stays disabled
    When they press Copy
    Then the button reads "Copied ✓", the clipboard holds the key and Continue unlocks
    When they press Continue
    Then the key is gone from the page and the success screen names only its first 8 characters

  Scenario: "I saved it" unlocks Continue without copying (key_setup_checkbox)
  Scenario: The site already has a key from register (key_setup_already_has_key)
    Then no second key is created and the success screen says the plugin already has its key
  Scenario: A second create is refused: reason key_exists (key_setup_409, key_settings_generate_409, key_err_key_exists, key_err_setup_reasons)
    When the server answers 409 {error, reason: "key_exists", message: "A plugin key already exists."}
    Then the page shows the server message "A plugin key already exists." followed by the next step: rotate it
    And in Settings focus moves to Rotate
  Scenario: Session missing or too old: reason reauth_required (key_err_reauth_required, key_err_reauth_stale, key_setup_sign_in_again, key_rotate_401_sign_in_again)
    When any plugin-key call answers 401 with reason "reauth_required"
    Then the page shows "Sign in again to continue.", the server message word for word, and "Nothing was changed."
    And a "Sign in again" link to the dashboard
  Scenario: A key instead of a session: reason forbidden_key_type (key_err_forbidden_key_type, key_err_setup_reasons, key_revoke_403)
    When any plugin-key call answers 403 with reason "forbidden_key_type"
    Then the page shows "This needs the dashboard, not a key.", the server message, and "Nothing was changed."
    And the link reads "Open the dashboard"
  Scenario: Other session refusal: reason forbidden (key_err_forbidden_other)
    Then the page shows "Sign in again to continue." with the server message
  Scenario: A body with only `error` (pre-2d3aed7) falls back to the v8 text (key_err_legacy_fallback)
    When a 401 has no reason, the page shows "Sign in again to continue." without a server line
    When a 409 has no reason, the page says "This site already has a plugin key." and that Rotate is the next step
  Scenario: Makers have no plugin key step (key_setup_maker_no_step)

  Scenario: Settings shows the prefix only (key_settings_rotate)
    Given a signed-in owner opens Settings for a site
    Then the panel shows "Active key starting <prefix>" and never the full key
  Scenario: Rotate (key_settings_rotate)
    When they press Rotate key
    Then an inline confirm says the current key stops working at once
    When they confirm
    Then the new key appears once with the same shown-once UI
  Scenario: Generate for an existing site without a key (key_settings_generate_201)
  Scenario: Revoke is a danger confirm (key_revoke_modal)
    When they press Revoke key…
    Then a modal says the plugin can't confirm placements any more and new boxes won't count as live until a new key is pasted
    And says boxes already live stay live, with focus on Cancel
    When they cancel, nothing is sent and the panel comes back
    When they confirm, POST /v1/sites/:id/plugin-key/revoke is sent and the panel says there is no active key
  Scenario: Key contexts (key_context_hidden)
    Given the digest was opened with a key (no session)
    Then Settings shows "Generating, rotating and revoking the plugin key need the dashboard" and no buttons, and no plugin-key call is made
  Scenario: CSRF on owner writes (csrf_on_writes, csrf_window_global_ignored)
    Then every non-GET call carries X-CSRF-Token read only from <meta name="csrf-token"> and no GET does
    And a window.OfferRails.csrf value is never sent (2d3aed7 sets no window global)
