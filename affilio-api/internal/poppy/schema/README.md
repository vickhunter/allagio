# poppy.json schema

Hand-written from Personal Agent Protocol draft 0.4. The spec does not publish a JSON Schema for this document.

- Spec: https://personalagentprotocol.org/docs/0.4/spec
- Version: `0.4` (`protocol_version`)
- Fields: section 3.1
- API types: section 7 (`openapi` and `mcp` only)

`additionalProperties` is false so a capability this host does not list fails validation. The spec tells a Personal Agent to ignore fields it does not know. This schema is the check for the document we serve.
