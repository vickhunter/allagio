# ORP reference host

In-process host and a thin client for the conformance suite. Party behavior is HTTP. The scheduler hooks (`advanceBusiness`, `runDueChecks`, `registerPage`, `markTestPool`, `failNextRecheck`, `resolveDispute`) are in-process on the reference host. The host speaks ORP 0.4 and 0.3. The Allagio hosted API speaks 0.4 and 0.3. `CLAIMS_CHECK` on that host stays a stub, so claim fields stay `unknown`.

`claims: "real"` checks destinations against the in-memory page catalog. It does not fetch a page and it is not the hosted `CLAIMS_CHECK` setting. `stub` and `none` leave an offer unchecked.

The provider test token and owner tokens are generated per host and kept in the local store. They are not production secrets.
