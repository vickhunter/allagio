Feature: Setup page after device-code sign-in (D-032)
  The agent scanned the project and signed the human in with a device code.
  The setup page asks at most 4 questions, all prefilled, so most people just press Continue.

  Background:
    Given the viewport is 1440 wide or 390 wide
    And every request to "**/v1/go/**" is aborted

  Scenario: Maker happy path, press Continue only
    Given the scan fixture "maker" from source "cli" for project "Rail Tool"
    When I open the setup page
    Then I see "Your agent is connected" with the source "cli" and "Rail Tool"
    And the role "Maker" is selected
    And I see at most 4 questions, all prefilled
    And "Approve anything that passes the checks" is preselected
    And the words "manual", "auto" and "yolo" do not appear on screen
    And I see the fixed list "Always checked, in every mode" with 7 checks
    When I press "Continue"
    Then an "offerrails:setup-submit" event fires with roles ["maker"], mode_shown "yolo" and mode_chosen "yolo"
    And I see the success state "Your agent can take it from here"

  Scenario: Publisher happy path
    Given the scan fixture "publisher" from source "cursor" for site "victormotricala.it"
    When I open the setup page and press "Continue"
    Then the event has roles ["publisher"], the found sites selected, box position "end" and mode_chosen "yolo"

  Scenario: Edit a prefilled field
    Given the scan fixture "maker"
    When I edit the listing pitch and choose "Ask me every time"
    And I press "Continue"
    Then the event carries the edited pitch, mode_shown "yolo", mode_chosen "manual" and edited_fields contains "listing.pitch"

  Scenario: Failed readiness signal is visible but does not block
    Given the scan fixture "maker" with the signal "Webhook verified" failing
    Then the listing card shows the score and the failing signal with its reason
    And "Continue" stays enabled

  Scenario: Scan found nothing for a field
    Given the scan fixture "empty"
    Then empty fields show a plain hint and Continue still submits

  Scenario: The server decides the preselected mode
    Given the scan fixture "directory" with mode_preselected "manual" from the server
    Then "Ask me every time" is preselected

  Scenario: No preselected mode from the server
    Given the scan fixture "nomode" with mode_preselected null
    Then no mode is preselected
    When I press "Continue"
    Then I am asked to pick a mode and no event fires

  Scenario: Error saving
    Given saving will fail
    When I press "Continue"
    Then I see "We couldn't save your settings" and my answers are kept

  # v9: 2d3aed7 defaults install_path from owners.source.
  Scenario: The page never forwards ?install_path= (real_setup_install_path)
    When setup opens with ?install_path=cli in the URL
    Then GET /v1/setup/settings carries only role (if any), and the save sends back the install_path the server returned
  Scenario: Preselected mode comes only from the server (mode_preselected_from_server)
    Then the checked mode is the server's mode_preselected or the saved mode, and with no mode_preselected nothing is checked, whatever the install path
