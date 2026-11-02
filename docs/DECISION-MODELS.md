# Decision models for Allagio

Research decision, 2026-09-30. Jev and CLM have not been installed, called, benchmarked or integrated into this POC. This note defines an optional experiment; it does not change runtime behavior or the commercial pilot gate.

## Recommendation

Evaluate a replaceable semantic decision layer for offer–content fit. Start with Jev if API access is available, because a hosted HTTP service avoids operating an encoder. Compare CLM when a suitable inference environment is available. Keep the existing deterministic matcher as the baseline and fallback. Neither provider should become a requirement for onboarding or the first paid pilot.

The current matcher uses lexical overlap and category signals. A semantic evaluator could recognize useful relationships without shared keywords and reject misleading overlap. This is a hypothesis to measure, not demonstrated improvement.

## Provider comparison

| Candidate | Documented capability | Practical implication |
|---|---|---|
| Jev | Hosted typed Choice, Score and Noul decisions. Current model `jev-1.13.0`; published input price $0.042/M tokens, outputs free. Early access. | Lower setup effort for an experiment. Pin the version; budget, time out and handle unavailable access. |
| CLM v0.1 | Apache-2.0 weights; separate state/action projections over Qwen3-8B embeddings; reusable candidate embeddings; TypeSafe-compatible HTTP interface. | Potentially useful for repeatedly ranking a stable offer catalog. Operating the encoder and cache adds infrastructure cost. |

[Jev API](https://docs.typesafe.ai/introduction/quickstart), [Jev model reference](https://docs.typesafe.ai/models), [TypeSafe release](https://typesafe.ai/blog/introducing-system-one-models-and-jev), [CLM model card](https://huggingface.co/Contrastive-LM/CLM-v0.1-8B), [CLM implementation](https://github.com/Contrastive-LM/CLM).

The CLM card currently lists no Hugging Face inference provider. Its strongest coding-verifier results use fine-tuned heads and do not establish affiliate-matching quality. Vendor speed comparisons depend on workload and hardware; measure end-to-end latency and operating cost here.

## Boundaries

The owner's existing agent continues to research, write copy and use authorized publishing tools. Our service can ask the decision model narrow questions about provided evidence:

- Does this product solve a problem described in this article?
- Is its documented intended user compatible with the article's intended reader?
- Does the proposed sentence contain a product claim unsupported by the supplied evidence?

Use separate judgments and explicit rubric levels. Include insufficient evidence and no suitable offer as real outcomes. A ranking winner alone is not evidence that any candidate deserves placement.

Typed output prevents invalid output shapes; it does not ensure a correct judgment. Jev documents problems with arithmetic, dates and adversarial content. CLM probabilities are relative to the supplied candidates. Provider confidence values are not interchangeable, are not an accuracy percentage, and are not a purchase probability. Calibrate each provider and question against our labels. [Jev limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13), [Confidence](https://docs.typesafe.ai/confidence), [CLM limitations](https://huggingface.co/Contrastive-LM/CLM-v0.1-8B#limitations).

Code retains control of credentials, listing eligibility, ownership, permissions, disclosure, expiration, attribution, commission arithmetic, refunds and settlement. A model cannot approve publication, authorize a payment or override these checks. Use only task-specific text; provider-bound experiments start with public or consented, redacted material and contain no credentials or buyer records.

## Experiment

1. Assemble 200 human-reviewed article–offer examples in the developer-tool niche. Include good matches, synonymous descriptions, misleading keyword overlap, missing evidence, unrelated high-commission offers, adversarial copy and cases where every candidate is unsuitable. Evaluate English and Italian separately when both are represented.
2. Freeze eligible candidate inventories and evidence snapshots. For the small pilot, evaluate all eligible offers so a lexical threshold cannot hide semantic matches. At larger scale, test retrieval recall separately before introducing a shortlist.
3. Split by product and article/site into development, calibration and held-out sets. Record rubric and model versions, evidence hashes and candidate IDs. Tune thresholds without using held-out labels.
4. Compare the current matcher, Jev and CLM on accepted-match precision at comparable coverage, missed good matches, correct abstention, unsupported-claim detection, provider-specific calibration, total cost and p50/p95 latency. Include cold and warm CLM caches and retries; report results by slice and sample size.
5. Run the better candidate beside the current workflow without changing recommendations or authority. Only enable it after held-out and shadow results show a useful improvement within the pilot's latency and operating budget. Failures and uncertain judgments return the existing baseline or an explicit no-match/review state.

Human editorial labels test suitability, not revenue lift. Learning which placements earn money requires observed attributable purchases, refunds and collected commissions. Model scores can later become features in an outcome model; they cannot substitute for those outcomes. Keep automatic bidding deferred until comparable commercial observations support it.

This experiment does not change the OpenAI public-plugin scope decision. Better decision models do not remove that directory's commerce and advertising restrictions. [OpenAI guidelines](https://developers.openai.com/plugins/plugin-guidelines#commerce-and-monetization).
